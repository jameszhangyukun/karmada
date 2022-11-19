package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/test/e2e/framework"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

var _ = ginkgo.Describe("Resource interpreter webhook testing", func() {
	var policyNamespace, policyName string
	var workloadNamespace, workloadName string
	var workload *workloadv1alpha1.Workload
	var policy *policyv1alpha1.PropagationPolicy

	ginkgo.BeforeEach(func() {
		policyNamespace = testNamespace
		policyName = workloadNamePrefix + rand.String(RandomStrLength)
		workloadNamespace = testNamespace
		workloadName = policyName

		workload = testhelper.NewWorkload(workloadNamespace, workloadName)
		policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: workload.APIVersion,
				Kind:       workload.Kind,
				Name:       workload.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: framework.ClusterNames(),
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateWorkload(dynamicClient, workload)
		ginkgo.DeferCleanup(func() {
			framework.RemoveWorkload(dynamicClient, workload.Namespace, workload.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
		})
	})

	ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
		ginkgo.It("InterpretReplica testing", func() {
			ginkgo.By("check if workload's replica is interpreted", func() {
				resourceBindingName := names.GenerateBindingName(workload.Kind, workload.Name)
				expectedReplicas := *workload.Spec.Replicas

				gomega.Eventually(func(g gomega.Gomega) (int32, error) {
					resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(workload.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof(fmt.Sprintf("ResourceBinding(%s/%s)'s replicas is %d, expected: %d.",
						resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.Replicas, expectedReplicas))
					return resourceBinding.Spec.Replicas, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(expectedReplicas))
			})
		})
	})

	ginkgo.Context("InterpreterOperation Retain testing", func() {
		var waitTime time.Duration
		var updatedPaused bool

		ginkgo.BeforeEach(func() {
			waitTime = 5 * time.Second
			updatedPaused = true

			policy.Spec.Placement.ClusterAffinity.ClusterNames = framework.ClusterNames()
		})

		ginkgo.It("Retain testing", func() {
			ginkgo.By("update workload's spec.paused to true", func() {
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Spec.Paused = updatedPaused
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster)
				}
			})

			// Wait executeController to reconcile then check if it is retained
			time.Sleep(waitTime)
			ginkgo.By("check if workload's spec.paused is retained", func() {
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					gomega.Eventually(func(g gomega.Gomega) (bool, error) {
						memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)

						return memberWorkload.Spec.Paused, nil
					}, pollTimeout, pollInterval).Should(gomega.Equal(updatedPaused))
				}
			})
		})
	})

	ginkgo.Context("InterpreterOperation ReviseReplica testing", func() {
		ginkgo.BeforeEach(func() {
			sumWeight := 0
			staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
			for index, clusterName := range framework.ClusterNames() {
				staticWeightList := policyv1alpha1.StaticClusterWeight{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Weight: int64(index + 1),
				}
				sumWeight += index + 1
				staticWeightLists = append(staticWeightLists, staticWeightList)
			}
			workload.Spec.Replicas = pointer.Int32Ptr(int32(sumWeight))
			policy = testhelper.NewPropagationPolicy(policyNamespace, policyName, []policyv1alpha1.ResourceSelector{
				{
					APIVersion: workload.APIVersion,
					Kind:       workload.Kind,
					Name:       workload.Name,
				},
			}, policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: staticWeightLists,
					},
				},
			})
		})

		ginkgo.It("ReviseReplica testing", func() {
			for index, clusterName := range framework.ClusterNames() {
				framework.WaitWorkloadPresentOnClusterFitWith(clusterName, workload.Namespace, workload.Name, func(workload *workloadv1alpha1.Workload) bool {
					return *workload.Spec.Replicas == int32(index+1)
				})
			}
		})
	})

	ginkgo.Context("InterpreterOperation AggregateStatus testing", func() {
		ginkgo.It("AggregateStatus testing", func() {
			ginkgo.By("check whether the workload status can be correctly collected", func() {
				// Simulate the workload resource controller behavior, update the status information of workload resources of member clusters manually.
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")
				}

				wantedReplicas := *workload.Spec.Replicas * int32(len(framework.Clusters()))
				klog.Infof("Waiting for workload(%s/%s) collecting correctly status", workloadNamespace, workloadName)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentWorkload := framework.GetWorkload(dynamicClient, workloadNamespace, workloadName)

					klog.Infof("workload(%s/%s) readyReplicas: %d, wanted replicas: %d", workloadNamespace, workloadName, currentWorkload.Status.ReadyReplicas, wantedReplicas)
					if currentWorkload.Status.ReadyReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("InterpreterOperation InterpretStatus testing", func() {
		ginkgo.It("InterpretStatus testing", func() {
			for _, cluster := range framework.ClusterNames() {
				clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
				gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())

				memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
				memberWorkload.Status.ReadyReplicas = *workload.Spec.Replicas
				framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")

				workName := names.GenerateWorkName(workload.Kind, workload.Name, workload.Namespace)
				workNamespace, err := names.GenerateExecutionSpaceName(cluster)
				gomega.Expect(err).Should(gomega.BeNil())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					work, err := karmadaClient.WorkV1alpha1().Works(workNamespace).Get(context.TODO(), workName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(work.Status.ManifestStatuses) == 0 || work.Status.ManifestStatuses[0].Status == nil {
						return false, nil
					}

					var observedStatus workloadv1alpha1.WorkloadStatus
					err = json.Unmarshal(work.Status.ManifestStatuses[0].Status.Raw, &observedStatus)
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof("work(%s/%s) readyReplicas: %d, want: %d", workNamespace, workName, observedStatus.ReadyReplicas, *workload.Spec.Replicas)

					// not collect status.conditions in webhook
					klog.Infof("work(%s/%s) length of conditions: %v, want: %v", workNamespace, workName, len(observedStatus.Conditions), 0)

					if observedStatus.ReadyReplicas == *workload.Spec.Replicas && len(observedStatus.Conditions) == 0 {
						return true, nil
					}
					return false, nil
				}, pollTimeout, pollInterval).Should(gomega.BeTrue())
			}
		})
	})

	ginkgo.Context("InterpreterOperation InterpretHealth testing", func() {
		ginkgo.It("InterpretHealth testing", func() {
			resourceBindingName := names.GenerateBindingName(workload.Kind, workload.Name)

			SetReadyReplicas := func(readyReplicas int32) {
				for _, cluster := range framework.ClusterNames() {
					clusterDynamicClient := framework.GetClusterDynamicClient(cluster)
					gomega.Expect(clusterDynamicClient).ShouldNot(gomega.BeNil())
					memberWorkload := framework.GetWorkload(clusterDynamicClient, workloadNamespace, workloadName)
					memberWorkload.Status.ReadyReplicas = readyReplicas
					framework.UpdateWorkload(clusterDynamicClient, memberWorkload, cluster, "status")
				}
			}

			CheckResult := func(result workv1alpha2.ResourceHealth) interface{} {
				return func(g gomega.Gomega) (bool, error) {
					rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(workload.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(rb.Status.AggregatedStatus) != len(framework.ClusterNames()) {
						return false, nil
					}

					for _, status := range rb.Status.AggregatedStatus {
						klog.Infof("resourceBinding(%s/%s) on cluster %s got %s, want %s ", workload.Namespace, resourceBindingName, status.ClusterName, status.Health, result)
						if status.Health != result {
							return false, nil
						}
					}
					return true, nil
				}
			}

			ginkgo.By("workload healthy", func() {
				SetReadyReplicas(*workload.Spec.Replicas)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceHealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})

			ginkgo.By("workload unhealthy", func() {
				SetReadyReplicas(1)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceUnhealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})
		})
	})
})

var _ = framework.SerialDescribe("Resource interpreter customization testing", func() {
	var customization *configv1alpha1.ResourceInterpreterCustomization
	var deployment *appsv1.Deployment
	var policy *policyv1alpha1.PropagationPolicy
	// We only need to test any one of the member clusters.
	var targetCluster string

	ginkgo.BeforeEach(func() {
		targetCluster = framework.ClusterNames()[rand.Intn(len(framework.ClusterNames()))]
		deployment = testhelper.NewDeployment(testNamespace, deploymentNamePrefix+rand.String(RandomStrLength))
		policy = testhelper.NewPropagationPolicy(testNamespace, deployment.Name, []policyv1alpha1.ResourceSelector{
			{
				APIVersion: deployment.APIVersion,
				Kind:       deployment.Kind,
				Name:       deployment.Name,
			},
		}, policyv1alpha1.Placement{
			ClusterAffinity: &policyv1alpha1.ClusterAffinity{
				ClusterNames: []string{targetCluster},
			},
		})
	})

	ginkgo.JustBeforeEach(func() {
		framework.CreateResourceInterpreterCustomization(karmadaClient, customization)
		// Wait for resource interpreter informer synced.
		time.Sleep(time.Second * 5)

		framework.CreatePropagationPolicy(karmadaClient, policy)
		framework.CreateDeployment(kubeClient, deployment)
		ginkgo.DeferCleanup(func() {
			framework.RemoveDeployment(kubeClient, deployment.Namespace, deployment.Name)
			framework.RemovePropagationPolicy(karmadaClient, policy.Namespace, policy.Name)
			framework.DeleteResourceInterpreterCustomization(karmadaClient, customization.Name)
		})
	})

	ginkgo.Context("InterpreterOperation InterpretReplica testing", func() {
		ginkgo.BeforeEach(func() {
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{
						LuaScript: `
function GetReplicas(desiredObj)
  replica = desiredObj.spec.replicas + 1
  requirement = {}
  requirement.nodeClaim = {}
  requirement.nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector
  requirement.nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations
  requirement.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits
  return replica, requirement
end`,
					},
				})
		})

		ginkgo.It("InterpretReplica testing", func() {
			ginkgo.By("check if workload's replica is interpreted", func() {
				resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)
				// Just for the current test case to distinguish the build-in logic.
				expectedReplicas := *deployment.Spec.Replicas + 1
				expectedReplicaRequirements := &workv1alpha2.ReplicaRequirements{
					ResourceRequest: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU: resource.MustParse("100m"),
					}}

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					resourceBinding, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					g.Expect(err).NotTo(gomega.HaveOccurred())

					klog.Infof(fmt.Sprintf("ResourceBinding(%s/%s)'s replicas is %d, expected: %d.",
						resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.Replicas, expectedReplicas))
					if resourceBinding.Spec.Replicas != expectedReplicas {
						return false, nil
					}

					klog.Infof(fmt.Sprintf("ResourceBinding(%s/%s)'s replicaRequirements is %+v, expected: %+v.",
						resourceBinding.Namespace, resourceBinding.Name, resourceBinding.Spec.ReplicaRequirements, expectedReplicaRequirements))
					return reflect.DeepEqual(resourceBinding.Spec.ReplicaRequirements, expectedReplicaRequirements), nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(true))
			})
		})
	})

	ginkgo.Context("InterpreterOperation ReviseReplica testing", func() {
		ginkgo.BeforeEach(func() {
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: `
function ReviseReplica(obj, desiredReplica)
  obj.spec.replicas = desiredReplica + 1
  return obj
end`,
					},
				})
		})

		ginkgo.BeforeEach(func() {
			sumWeight := 0
			staticWeightLists := make([]policyv1alpha1.StaticClusterWeight, 0)
			for index, clusterName := range framework.ClusterNames() {
				staticWeightList := policyv1alpha1.StaticClusterWeight{
					TargetCluster: policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{clusterName},
					},
					Weight: int64(index + 1),
				}
				sumWeight += index + 1
				staticWeightLists = append(staticWeightLists, staticWeightList)
			}
			deployment.Spec.Replicas = pointer.Int32Ptr(int32(sumWeight))
			policy.Spec.Placement = policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: framework.ClusterNames(),
				},
				ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
					ReplicaDivisionPreference: policyv1alpha1.ReplicaDivisionPreferenceWeighted,
					ReplicaSchedulingType:     policyv1alpha1.ReplicaSchedulingTypeDivided,
					WeightPreference: &policyv1alpha1.ClusterPreferences{
						StaticWeightList: staticWeightLists,
					},
				},
			}
		})

		ginkgo.It("ReviseReplica testing", func() {
			for index, clusterName := range framework.ClusterNames() {
				framework.WaitDeploymentPresentOnClusterFitWith(clusterName, deployment.Namespace, deployment.Name, func(deployment *appsv1.Deployment) bool {
					return *deployment.Spec.Replicas == int32(index+1)+1
				})
			}
		})
	})

	ginkgo.Context("InterpreterOperation Retain testing", func() {
		var waitTime time.Duration
		var updatedPaused bool

		ginkgo.BeforeEach(func() {
			waitTime = 5 * time.Second
			updatedPaused = true
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},

				configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: `function Retain(desiredObj, observedObj)
			desiredObj.spec.paused = observedObj.spec.paused
			return desiredObj   
			end`,
					},
				})
		})

		ginkgo.It("Retain testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{targetCluster}, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})
			ginkgo.By("update workload's spec.paused to true", func() {
				clusterClient := framework.GetClusterClient(targetCluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				memberDeploy := framework.GetDeployment(clusterClient, deployment.Namespace, deployment.Name)
				memberDeploy.Spec.Paused = updatedPaused
				klog.Infof("Update memberDeploy %v", memberDeploy)
				framework.UpdateDeployment(clusterClient, memberDeploy)
			})

			// Wait executeController to reconcile then check if it is retained
			time.Sleep(waitTime)
			ginkgo.By("check if workload's spec.paused is retained", func() {
				clusterClient := framework.GetClusterClient(targetCluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				gomega.Eventually(func(g gomega.Gomega) (bool, error) {
					memberDeployment := framework.GetDeployment(clusterClient, deployment.Namespace, deployment.Name)
					klog.Infof("Get memberDeploy %v", memberDeployment)
					return memberDeployment.Spec.Paused, nil
				}, pollTimeout, pollInterval).Should(gomega.Equal(updatedPaused))
			})
		})
	})
	ginkgo.Context("InterpreterOperation AggregateStatus testing", func() {
		ginkgo.BeforeEach(func() {
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: `
function AggregateStatus(desiredObj, statusItems) 
										for i = 1, #statusItems do    
											desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + statusItems[i].status.readyReplicas 
										end    
										return desiredObj
									end`,
					},
				})
		})
		ginkgo.It("AggregateStatus testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{targetCluster}, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})
			ginkgo.By("check whether the deployment status can be correctly collected", func() {
				// Simulate the workload resource controller behavior, update the status information of workload resources of member clusters manually.
				clusterClient := framework.GetClusterClient(targetCluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

				memberDeployment := framework.GetDeployment(clusterClient, deployment.Namespace, deployment.Name)
				memberDeployment.Status.ReadyReplicas = *deployment.Spec.Replicas
				klog.Infof("UpdateDeployment memberDeploy %v", memberDeployment)
				framework.UpdateDeployment(clusterClient, memberDeployment)

				wantedReplicas := *deployment.Spec.Replicas * int32(len(framework.Clusters()))
				klog.Infof("Waiting for deployment(%s/%s) collecting correctly status", deployment.Namespace, deployment.Name)
				err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
					currentDeployment := framework.GetDeployment(kubeClient, deployment.Namespace, deployment.Name)
					klog.Infof("deployment(%s/%s) readyReplicas: %d, wanted replicas: %d", deployment.Namespace, deployment.Name, currentDeployment.Status.ReadyReplicas, wantedReplicas)
					if currentDeployment.Status.ReadyReplicas == wantedReplicas {
						return true, nil
					}

					return false, nil
				})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Context("InterpreterOperation InterpretStatus testing", func() {
		ginkgo.BeforeEach(func() {
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: `
function ReflectStatus (observedObj)
						if observedObj.status == nil then	
							return nil   
						end    
					return observedObj.status
					end`,
					},
				})
		})
		ginkgo.It("InterpretStatus testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{targetCluster}, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})

			clusterClient := framework.GetClusterClient(targetCluster)
			gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())

			memberDeployment := framework.GetDeployment(clusterClient, deployment.Namespace, deployment.Name)
			memberDeployment.Status.ReadyReplicas = *deployment.Spec.Replicas
			klog.Infof("memberDeployment %v", memberDeployment)
			framework.UpdateDeployment(clusterClient, memberDeployment)

			gomega.Eventually(func(g gomega.Gomega) (bool, error) {
				deploy, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
				g.Expect(err).NotTo(gomega.HaveOccurred())
				klog.Infof("deploy %v", deploy)
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas && len(deploy.Status.Conditions) == 0 {
					return true, nil
				}
				return false, nil
			}, pollTimeout, pollInterval).Should(gomega.BeTrue())

		})
	})

	ginkgo.Context("InterpreterOperation InterpretHealth testing", func() {
		ginkgo.BeforeEach(func() {
			customization = testhelper.NewResourceInterpreterCustomization(
				"interpreter-customization"+rand.String(RandomStrLength),
				configv1alpha1.CustomizationTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				configv1alpha1.CustomizationRules{
					ReplicaRevision: &configv1alpha1.ReplicaRevision{
						LuaScript: `function InterpretHealth(observedObj)
							return (observedObj.status.updatedReplicas == observedObj.spec.replicas) and (observedObj.metadata.generation == observedObj.status.observedGeneration)
                        end `,
					},
				})
		})
		ginkgo.It("InterpretHealth testing", func() {
			framework.WaitDeploymentPresentOnClustersFitWith([]string{targetCluster}, deployment.Namespace, deployment.Name,
				func(deployment *appsv1.Deployment) bool {
					return true
				})
			resourceBindingName := names.GenerateBindingName(deployment.Kind, deployment.Name)

			SetReadyReplicas := func(readyReplicas int32) {
				clusterClient := framework.GetClusterClient(targetCluster)
				gomega.Expect(clusterClient).ShouldNot(gomega.BeNil())
				memberDeployment := framework.GetDeployment(clusterClient, deployment.Namespace, deployment.Name)
				memberDeployment.Status.ReadyReplicas = readyReplicas
				klog.Infof("memberDeployment %v", memberDeployment)
				framework.UpdateDeployment(clusterClient, memberDeployment)
			}

			CheckResult := func(result workv1alpha2.ResourceHealth) interface{} {
				return func(g gomega.Gomega) (bool, error) {
					rb, err := karmadaClient.WorkV1alpha2().ResourceBindings(deployment.Namespace).Get(context.TODO(), resourceBindingName, metav1.GetOptions{})
					klog.Infof("rb %v", rb)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if len(rb.Status.AggregatedStatus) != 1 {
						return false, nil
					}

					for _, status := range rb.Status.AggregatedStatus {
						klog.Infof("AggregatedStatus %v", status)
						klog.Infof("resourceBinding(%s/%s) on cluster %s got %s, want %s ", deployment.Namespace, resourceBindingName, status.ClusterName, status.Health, result)
						if status.Health != result {
							return false, nil
						}
					}
					return true, nil
				}
			}

			ginkgo.By("deployment healthy", func() {
				SetReadyReplicas(*deployment.Spec.Replicas)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceHealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})

			ginkgo.By("deployment unhealthy", func() {
				SetReadyReplicas(1)
				gomega.Eventually(CheckResult(workv1alpha2.ResourceUnhealthy), pollTimeout, pollInterval).Should(gomega.BeTrue())
			})
		})
	})
})

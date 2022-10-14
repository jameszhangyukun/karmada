nodeClaim = {}
resourceRequest = {}
result = {}
if object.status.replicas ~= nil then
  result.replica = object.status.replicas
end


nodeClaim.nodeSelector = object.spec.template.spec.nodeSelector
nodeClaim.tolerations =  object.spec.template.spec.tolerations


if object.spec.template.spec.nodeSelector == nil and object.spec.template.spec.hardNodeAffinity == nil and #object.spec.template.spec.tolerations == 0 then
   nodeClaim = nil
end

if #object.spec.template.spec.containers > 0 then
    resourceRequest = object.spec.template.spec.containers[1].resources.limits
end


result.nodeClaim = nodeClaim
result.resourceRequest  = resourceRequest
return result
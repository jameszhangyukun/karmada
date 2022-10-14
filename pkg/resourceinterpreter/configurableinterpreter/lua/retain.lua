desired.spec.nodeName = observed.spec.nodeName
desired.spec.serviceAccountName = observed.spec.serviceAccountName
desired.spec.volumes = observed.spec.volumes

for i = 1, #observed.spec.containers do
	for j = 1, #desired.spec.containers do
	    if observed.spec.containers[i].name == desired.spec.containers[i].name then
            desired.spec.containers[i].volumeMounts = observed.spec.containers[i].volumeMounts
	    end
	end
end



return desired
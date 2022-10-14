dependentSas = {}
refs = {}

if object.spec.serviceAccountName ~= "" and object.spec.serviceAccountName ~= "default" then
    dependentSas[object.spec.serviceAccountName] = true
end

local idx = 1

for key, value in pairs(dependentSas) do
    dependObj = {}
    dependObj.apiVersion = "v1"
    dependObj.kind = "ServiceAccount"
    dependObj.name = key
    dependObj.namespace = object.namespace
    refs[idx] = {}
    refs[idx] = dependObj
    idx = idx + 1
end

return refs
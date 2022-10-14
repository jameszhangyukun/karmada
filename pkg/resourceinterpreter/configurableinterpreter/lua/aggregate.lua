print(#items)
for i = 1, #items do
    if object.status.replicas==nil then
        object.status.replicas= 0
    end
    object.status["replicas"] = object.status["replicas"]  +  items[i].replicas

    if object.status.readyReplicas==nil then
            object.status.readyReplicas= 0
        end

    object.status.readyReplicas = object.status.readyReplicas +  items[i].readyReplicas

   if object.status.updatedReplicas==nil then
           object.status.updatedReplicas= 0
   end

    object.status.updatedReplicas = object.status.updatedReplicas +  items[i].updatedReplicas

    if object.status.availableReplicas==nil then
               object.status.availableReplicas= 0
       end
    object.status.availableReplicas = object.status.availableReplicas + items[i].availableReplicas
        if object.status.unavailableReplicas==nil then
                   object.status.unavailableReplicas= 0
           end
    object.status["unavailableReplicas"]  = object.status["unavailableReplicas"]+ items[i].unavailableReplicas
end

return object
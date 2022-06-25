    
	if alertManagerDefinitionCreating(&resource{ko}) {
		return &resource{ko}, requeueWaitWhileCreating
	}
    if alertManagerDefinitionUpdating(&resource{ko}) {
		return &resource{ko}, requeueWaitWhileUpdating
	}

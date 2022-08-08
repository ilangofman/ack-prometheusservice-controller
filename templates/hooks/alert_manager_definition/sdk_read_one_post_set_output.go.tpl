	
	// If in the middle of updating, requeue again
	if alertManagerDefinitionUpdating(&resource{ko}) {
        return &resource{ko}, requeueWaitWhileUpdatingWithoutError
	}
	
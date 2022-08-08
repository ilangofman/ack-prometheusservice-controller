
    // Check the status of the alert manager definition
    if resp.AlertManagerDefinition.Status != nil {
        if resp.AlertManagerDefinition.Status.StatusCode != nil {
            ko.Status.StatusCode = resp.AlertManagerDefinition.Status.StatusCode
        }else{
            ko.Status.StatusCode = nil
        }
        if resp.AlertManagerDefinition.Status.StatusReason != nil {
            ko.Status.StatusReason = resp.AlertManagerDefinition.Status.StatusReason
        }else{
            ko.Status.StatusReason = nil
        }
    } else {
        ko.Status.StatusCode = nil
        ko.Status.StatusReason = nil

    }
 
    // When adding an invalid alert manager configuration, the AMP API has different behaviour 
    // for different kinds of invalid input. For some invalid input, the API returns an error
    // (e.g. ValidationException) instantly and we set the controller to terminal state. The specified 
    // spec remains the same. 
    // For other invalid inputs, the API first returns a 200, and proceeds to create the configuration but 
    // ultimately fails after around a minute because of an invalid config. In this case, the 

    // If there was a failed creation or failed udpate of the alert manager configuration
    // then the API behaviour will be 
    // We set the resource to terminal state because it will require human intervention to resolve 
    // 

	// So for failed cases, we leave the spec to be what the user desires. 

    // IF STATUS FAILED AND NOT NOT NOT already terminal status then only do this
    if (alertManagerDefinitionStatusFailed(&resource{ko}) && !alertManagerDefinitionStatusFailed(r)){
	 	msg := "Alert Manager Definition is in '" + *ko.Status.StatusCode + "' status"
        ackcondition.SetTerminal(&resource{ko}, corev1.ConditionTrue, &msg, nil)
        ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)

        rm.setStatusDefaults(ko)
        return &resource{ko}, nil

    }

    
    // The data field stores the base64 encoding of the alert manager definition.
    // However, to make the CR's more user friendly, we convert the base64 encoding to a 
    // string. We store it in a custom created field. 
    if resp.AlertManagerDefinition.Data != nil {
        // Convert the base64 byte array to a human-readable string
        alertManagerDefinitionDataString := string(resp.AlertManagerDefinition.Data)
        ko.Spec.AlertmanagerConfig = &alertManagerDefinitionDataString
        if err != nil {
            return nil, err
        }
        // Remove the data field as it is not user facing
        resp.AlertManagerDefinition.Data = nil
    } else {
        ko.Spec.AlertmanagerConfig = nil
    }

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
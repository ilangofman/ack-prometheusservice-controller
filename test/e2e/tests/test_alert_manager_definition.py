# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the Amazon Managed Prometheus (AMP) Alert Manager Definitions resource
"""

import logging
import time
from urllib import response
import pytest
import sys #TODO ilan remove

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_prometheusservice_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e import condition
import boto3


RESOURCE_KIND = "alertmanagerdefinition"
RESOURCE_PLURAL = "alertmanagerdefinitions"

CREATE_WAIT_AFTER_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_WAIT_FOR_SYNCED_MINUTES = 10
UPDATE_WAIT_AFTER_SECONDS = 20


def create_sns_topic(topic_name) -> str:
    try:
        client = boto3.client('sns')
        resp = client.create_topic(
            Name=topic_name
        )

        return resp['TopicArn']

    except Exception as e:
        logging.debug(e)
        return None

def delete_sns_topic(topic_arn):
    try:
        client = boto3.client('sns')
        resp = client.delete_topic(
            TopicArn=topic_arn
        )
        print("ILAN DELETING SNS: ",resp, flush=True)
        logging.debug("ILAN DELETING SNS: ",resp)

    except Exception as e:
        logging.debug(e)
        return None

@service_marker
@pytest.mark.canary
class TestAlertManagerDefinition:

    def create_workspace(self, prometheusservice_client, workspace_alias) -> str:
        try:
            resp = prometheusservice_client.create_workspace(
                alias=workspace_alias
            )
            print("ILAN IN CREATE WORKSPACE ---->", resp, flush=True,file=sys.stdout)
            # logging.debug("create workspace ilan respo", str(resp))
            workspace_id = resp['workspaceId']
            # logging.debug("1WORKSPACE ILAN", workspace_id)
            # print("ILAN WORKSPACE ID --1->", workspace_id,file=sys.stdout, flush=True )

            return workspace_id

        except Exception as e:
            print("ERROR ILAN1 --> ", e,file=sys.stdout, flush=True)
            logging.debug(e)
            return None

    def delete_workspace(self, prometheusservice_client, workspace_id):
        try:
            response = prometheusservice_client.delete_workspace(
                workspaceId=workspace_id
            )
            print("ILAN DELETING workspace: ", workspace_id, "---",response, file=sys.stdout, flush=True)
            logging.debug("ILAN DELETING workspace: ",response)
            return response
        except Exception as e:
            print("ERROR ILAN --> ", e,file=sys.stdout, flush=True)
            logging.debug(e)
            return None

    def get_alert_manager_definition(self, prometheusservice_client, workspaceID: str) -> dict:
        try:
            resp = prometheusservice_client.describe_alert_manager_definition(
                workspaceId=workspaceID
            )
            return resp

        except Exception as e:
            print("ERROR ILAN --> ", e,file=sys.stdout, flush=True)
            logging.debug(e)
            return None

    def test_successful_create_alert_manager_definition(self, prometheusservice_client):
        # TODO change to more than 0 
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name_prefix
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        workspace_alias = random_suffix_name("amp-workspace", 0) 

        resource_name = random_suffix_name("alert-manager-definition", 30)


        logging.debug("Ilan Starting new test...")
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        print("ILAN WORKSPACE ID --->", workspace_id,file=sys.stdout, flush=True )


        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['SNS_TOPIC_NAME'] = sns_topic_name
        replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name


           
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        print("ERROR ILAN -3-> ", am_resource,file=sys.stdout, flush=True)
        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        # assert 'workspaceID' in am_resource['status']
        condition.assert_not_synced(am_ref)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        print("ERROR ILAN -4-> ", latest,file=sys.stdout, flush=True)
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'

        # Before we update the workspace CR below, we need to check that the
        # workspace status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(am_ref)

        new_alert_config = '''alertmanager_config: |
  route:
    receiver: '{SNS_TOPIC_NAME}'
  receivers:
    - name: '{SNS_TOPIC_NAME}'
      sns_configs:
      - topic_arn: {SNS_TOPIC_ARN}
        sigv4:
          region: us-west-2
        attributes:
          key: key2
          value: value2'''.format(**replacements)
        # logging.debug("Ilan workspaceID ", str(workspace_id))
        
        print("ERROR ILAN -6-> ", new_alert_config,file=sys.stdout, flush=True)

        print("ERROR ILAN -12-> ", am_resource['spec']['alertmanagerConfig'],file=sys.stdout, flush=True)


        updates = {
            "spec": {"alertmanagerConfig": new_alert_config},
        }



        res= k8s.patch_custom_resource(am_ref, updates)

        print("ERROR ILAN -7-> ", res,file=sys.stdout, flush=True)
        print("ERROR ILAN -9-> statsu after update ", am_resource['status']['statusCode'],file=sys.stdout, flush=True)

        # A successful update could take a little while to complete. 
        # As a intermediate step, the status should be updated to "UPDATING"
        # shorly after the update call was made. 
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        print("ERROR ILAN -10-> ", res,file=sys.stdout, flush=True)
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'UPDATING'

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        print("ERROR ILAN -11-> ", latest,file=sys.stdout, flush=True)
        # TODO check that the alert manager definition was updated 
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        # Since it is base64 encoded, the responding configuration will be in bytes and needs to be converted 
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == new_alert_config
        # print("ERROR ILAN -14-> ", latest['alertManagerDefinition']['data'].decode('UTF-8'),file=sys.stdout, flush=True)
        # print("ERROR ILAN -15-> ", new_alert_config,file=sys.stdout, flush=True)



        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted

        # Cleanup 
        self.delete_workspace(prometheusservice_client, workspace_id)
        delete_sns_topic(sns_topic_arn)
        # assert True == False

    def test_failed_alert_manager_creation(self, prometheusservice_client):
        # TODO change to more than 0 
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name_prefix
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        workspace_alias = random_suffix_name("amp-workspace", 0) 
        # sns_topic_name = random_suffix_name("ACK-AMP-Test-Topic", 24)
        resource_name = random_suffix_name("alert-manager-definition", 30)


        logging.debug("Ilan Starting new test...")
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        print("ILAN WORKSPACE ID --->", workspace_id,file=sys.stdout, flush=True )

        # sns_topic_arn = create_sns_topic(sns_topic_name)
        # time.sleep(20)

        logging.info("Ilan in test...")


        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['SNS_TOPIC_NAME'] = sns_topic_name
        replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name

        # print("ILAN replacement vlaues ID --->", replacements,file=sys.stdout, flush=True )

           
        resource_data = load_prometheusservice_resource(
            "invalid_alert_manager_definition",
            additional_replacements=replacements,
        )
        # print("ERROR ILAN -2-> ", resource_data,file=sys.stdout, flush=True)

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        print("ERROR ILAN -3-> ", am_resource,file=sys.stdout, flush=True)
        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        # assert 'workspaceID' in am_resource['status']
        condition.assert_not_synced(am_ref)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        print("ERROR ILAN -4-> ", latest,file=sys.stdout, flush=True)
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'CREATION_FAILED'

        # Before we update the workspace CR below, we need to check that the
        # workspace status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATION_FAILED'
        condition.assert_synced(am_ref)

    

        new_alert_config = '''alertmanager_config: |
  route:
    receiver: '{SNS_TOPIC_NAME}'
  receivers:
    - name: '{SNS_TOPIC_NAME}'
      sns_configs:
      - topic_arn: {SNS_TOPIC_ARN}
        sigv4:
          region: us-west-2
        attributes:
          key: key2
          value: value2'''.format(**replacements)
        # logging.debug("Ilan workspaceID ", str(workspace_id))
        
        print("ERROR ILAN -6-> ", new_alert_config,file=sys.stdout, flush=True)

        print("ERROR ILAN -12-> ", am_resource['spec']['alertmanagerConfig'],file=sys.stdout, flush=True)


        updates = {
            "spec": {"alertmanagerConfig": new_alert_config},
        }



        res= k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        print("ERROR ILAN -11-> ", latest,file=sys.stdout, flush=True)
        # TODO check that the alert manager definition was updated 
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        # Since it is base64 encoded, the responding configuration will be in bytes and needs to be converted 
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == new_alert_config
        # print("ERROR ILAN -14-> ", latest['alertManagerDefinition']['data'].decode('UTF-8'),file=sys.stdout, flush=True)
        # print("ERROR ILAN -15-> ", new_alert_config,file=sys.stdout, flush=True)



        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted

        # Cleanup 
        self.delete_workspace(prometheusservice_client, workspace_id)
        delete_sns_topic(sns_topic_arn)


    def test_failed_alert_manager_update(self, prometheusservice_client):
        # TODO change to more than 0 
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name_prefix
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        workspace_alias = random_suffix_name("amp-workspace", 0) 
        # sns_topic_name = random_suffix_name("ACK-AMP-Test-Topic", 24)
        resource_name = random_suffix_name("alert-manager-definition", 30)


        logging.debug("Ilan Starting new test...")
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        print("ILAN WORKSPACE ID --->", workspace_id,file=sys.stdout, flush=True )

        # sns_topic_arn = create_sns_topic(sns_topic_name)
        # time.sleep(20)

        logging.info("Ilan in test...")


        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['SNS_TOPIC_NAME'] = sns_topic_name
        replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name

        # print("ILAN replacement vlaues ID --->", replacements,file=sys.stdout, flush=True )

           
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )
        # print("ERROR ILAN -2-> ", resource_data,file=sys.stdout, flush=True)

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        print("ERROR ILAN -3-> ", am_resource,file=sys.stdout, flush=True)
        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        # assert 'workspaceID' in am_resource['status']
        condition.assert_not_synced(am_ref)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        print("ERROR ILAN -4-> ", latest,file=sys.stdout, flush=True)
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'

        # Before we update the workspace CR below, we need to check that the
        # workspace status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(am_ref)

    

        new_alert_config = '''alertmanager_config: |
  #route:
  #  receiver: '{SNS_TOPIC_NAME}'
  receivers:
    - name: '{SNS_TOPIC_NAME}'
      sns_configs:
      - topic_arn: {SNS_TOPIC_ARN}
        sigv4:
          region: us-west-2
        attributes:
          key: key2
          value: value2'''.format(**replacements)
        # logging.debug("Ilan workspaceID ", str(workspace_id))
        
        print("ERROR ILAN -6-> ", new_alert_config,file=sys.stdout, flush=True)

        print("ERROR ILAN -12-> ", am_resource['spec']['alertmanagerConfig'],file=sys.stdout, flush=True)


        updates = {
            "spec": {"alertmanagerConfig": new_alert_config},
        }



        res= k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        print("ERROR ILAN -11-> ", latest,file=sys.stdout, flush=True)
        # TODO check that the alert manager definition was updated 
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'UPDATE_FAILED'
        assert 'data' in latest['alertManagerDefinition']
        # Since it is base64 encoded, the responding configuration will be in bytes and needs to be converted 
        # TODO 
        # assert latest['alertManagerDefinition']['data'].decode('UTF-8') == new_alert_config
        # print("ERROR ILAN -14-> ", latest['alertManagerDefinition']['data'].decode('UTF-8'),file=sys.stdout, flush=True)
        # print("ERROR ILAN -15-> ", new_alert_config,file=sys.stdout, flush=True)

        #TODO check terminal errro


        new_alert_config = '''alertmanager_config: |
  route:
    receiver: '{SNS_TOPIC_NAME}'
  receivers:
    - name: '{SNS_TOPIC_NAME}'
      sns_configs:
      - topic_arn: {SNS_TOPIC_ARN}
        sigv4:
          region: us-west-2
        attributes:
          key: key2
          value: value2'''.format(**replacements)
        # logging.debug("Ilan workspaceID ", str(workspace_id))
        
        print("ERROR ILAN -6-> ", new_alert_config,file=sys.stdout, flush=True)

        print("ERROR ILAN -12-> ", am_resource['spec']['alertmanagerConfig'],file=sys.stdout, flush=True)


        updates = {
            "spec": {"alertmanagerConfig": new_alert_config},
        }



        res= k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)



        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        print("ERROR ILAN -11-> ", latest,file=sys.stdout, flush=True)
        # TODO check that the alert manager definition was updated 
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'

        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted

        # Cleanup 
        self.delete_workspace(prometheusservice_client, workspace_id)
        delete_sns_topic(sns_topic_arn)


    def test_creating_two_alert_manager_for_one_workspace(self, prometheusservice_client):
        # Todo, if a second one is created, it should not be adopted 
        # TODO change to more than 0 
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name_prefix
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        workspace_alias = random_suffix_name("amp-workspace", 0) 
        # sns_topic_name = random_suffix_name("ACK-AMP-Test-Topic", 24)
        resource_name = random_suffix_name("alert-manager-definition", 30)


        logging.debug("Ilan Starting new test...")
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        print("ILAN WORKSPACE ID --->", workspace_id,file=sys.stdout, flush=True )

        # sns_topic_arn = create_sns_topic(sns_topic_name)
        # time.sleep(20)

        logging.info("Ilan in test...")


        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['SNS_TOPIC_NAME'] = sns_topic_name
        replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name

        # print("ILAN replacement vlaues ID --->", replacements,file=sys.stdout, flush=True )

        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )
        # print("ERROR ILAN -2-> ", resource_data,file=sys.stdout, flush=True)

        am_ref_1 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(am_ref_1, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref_1)

        print("ERROR ILAN -3-> ", am_resource,file=sys.stdout, flush=True)
        assert k8s.get_resource_exists(am_ref_1)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        # assert 'workspaceID' in am_resource['status']
        condition.assert_not_synced(am_ref_1)

        time.sleep(1)


        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name + '-new'
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )
        am_ref_2 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name + '-new', namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(am_ref_2, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref_2)

        print("ERROR ILAN -3-> ", am_resource,file=sys.stdout, flush=True)
        assert k8s.get_resource_exists(am_ref_1)
        assert k8s.get_resource_exists(am_ref_2)
        
        time.sleep(120)

        # assert k8s.wait_on_condition(am_ref_1, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        # assert k8s.wait_on_condition(am_ref_2, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        am_resource1 = k8s.get_resource(am_ref_1)
        am_resource2 = k8s.get_resource(am_ref_2)
        print("ERROR ILAN -6-> ", am_resource1,file=sys.stdout, flush=True)
        print("ERROR ILAN -67-> ", am_resource2,file=sys.stdout, flush=True)

        condition.assert_synced(am_ref_1)
        condition.assert_type_status(am_ref_2, condition.CONDITION_TYPE_TERMINAL, True)
        condition.assert_not_synced(am_ref_2)

        _, deleted = k8s.delete_custom_resource(am_ref_2)
        assert deleted

        # TODO make sure that the alert manager definition still exists for the workspace
        _, deleted = k8s.delete_custom_resource(am_ref_1)
        assert deleted
#         # Cleanup 
        self.delete_workspace(prometheusservice_client, workspace_id)
        delete_sns_topic(sns_topic_arn)

# Put resused code into fucntions with the asserts 
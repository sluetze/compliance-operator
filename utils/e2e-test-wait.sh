#!/bin/bash

# Wait up to two minutes for compliance operator CRDs to cleanup.
i=0
w=10
while [[ $i -lt 12 ]]; do
        if [[ $(oc api-resources --api-group=compliance.openshift.io --no-headers) ]]; then
                echo "compliance.openshift.io CRDs still exist..."
                sleep $w
                i=$i+1
                continue
        fi
        echo "no compliance.openshift.io CRDs left in deployment"
        exit
done
echo "timed out waiting for compliance.openshift.io CRDs to cleanup"
exit 1

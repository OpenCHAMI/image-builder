#!/bin/bash

if [[ ! -v USERNS_RANGE ]]
then
	USERNS_RANGE=64536
fi


echo "builder:1001:${USERNS_RANGE}" > /etc/subuid
echo "builder:1001:${USERNS_RANGE}" > /etc/subgid

chown -R builder /home/builder

exec su builder -c "buildah unshare $*"

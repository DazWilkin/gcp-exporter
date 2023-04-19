#!/usr/bin/env bash

set -x

# trap read debug

# Environment file
export $(cat .env.test | xargs)

function update_images_dazwilkin () {
    echo "update_images_dazwilkin"

    local TOKEN=${GHCR_TOKEN}

    local IMAGES=(
      ${IMAGE_AZURE_EXPORTER}
      ${IMAGE_FLY_EXPORTER}
      ${IMAGE_GCP_STATUS}
      ${IMAGE_GCP_EXPORTER}
      ${IMAGE_LINODE_EXPORTER}
      ${IMAGE_VULTR_EXPORTER}
    )

    # Filter is explained in update_images_brabantcourt
    local SIGSTORE="sha256-[0-9a-z]{64}.sig"
    local FILTER="
        [
            .[0:2]
            | .[]
            | .metadata.container.tags[0]
        ]
        | if (
            .[0]
            |test(\"${SIGSTORE}\")
            )
            then .[1] 
            else .[0]
            end
    "

    for IMAGE in "${IMAGES[@]}"
    do
        # These images include ghcr.io/dazwilkin repo prefix
        # Remove the repo prefix
        IMAGE=${IMAGE#ghcr.io/dazwilkin/}

        # Split IMAGE into NAME:TAG
        IFS=':' read NAME TAG <<< "${IMAGE}"

        # Use GitHub REST API to find latest (most recent; 0th) tag
        LATEST=$(curl \
        --silent \
        --header "Authorization: Bearer ${TOKEN}" \
        https://api.github.com/users/dazwilkin/packages/container/${NAME}/versions \
        | jq -r "${FILTER}")
        
        printf "%s " ${NAME}

        if [ "${TAG}" != "${LATEST}" ]
        then
            printf "[Updating: %s --> %s]\n" ${TAG} ${LATEST}
            sed --in-place \
            --expression "s|${IMAGE}|${NAME}:${LATEST}|g" \
            .env.test
        else
            printf "[Current]\n"
        fi
    done
}

function update_images_dockerhub () {
    echo "update_images_dockerhub"

    local TOKEN=${DOCKERHUB_API_KEY}

    local ENDPOINT="https://hub.docker.com/v2"

    local IMAGES=(
        ${IMAGE_ALERTMANAGER}
        ${IMAGE_PROMETHEUS}
    )

    for IMAGE in "${IMAGES[@]}"
    do
        # IMAGE = docker.io/{NAMESPACE}/{REPOSITORY}:{TAG}
        # Remove docker.io/
        IMAGE=${IMAGE#docker.io/}
        # Extract text before / which is {NAMESPACE}
        NAMESPACE=${IMAGE%/*}
        # Extract text after / which is {REPOSITORY}:{TAG}
        REPOTAG=${IMAGE#*/}
        # Extract text before : which is {REPOSITORY}
        REPOSITORY=${REPOTAG%:*}
        # Extract text after : which is {TAG}
        TAG=${REPOTAG#*:}

        # GitHub API List Repository Tags
        # https://docs.docker.com/docker-hub/api/latest/#tag/repositories/paths/~1v2~1namespaces~1%7Bnamespace%7D~1repositories~1%7Brepository%7D~1tags/get
        # jq command extracts first page (most recent?) list of 10 tags
        # sed #1 excludes lines that don't match (!d) (v)X.X.X eliminating e.g. latest, main, v3.4.5-rc0 etc.
        # sort uses version-sort capability but reverses the results so that most recent is first
        # head extracts first most recent tag which should be the most recent numerical tag
        LATEST=$(\
            curl \
            --silent \
            --get \
            --header "Authentication: Bearer {$TOKEN}" \
            ${ENDPOINT}/namespaces/${NAMESPACE}/repositories/${REPOSITORY}/tags \
            | jq -r ".results[].name" \
            | sed --expression '/^v*[0-9]\{1,3\}\.[0-9]\{1,3\}\.[0-9]\{1,3\}$/!d' \
            | sort --version-sort --reverse \
            | head -n 1)
        
        printf "%s/%s " ${NAMESPACE} ${REPOSITORY}

        # # Update environment file if {TAG}!={LATEST}
        if [ "${TAG}" != "${LATEST}" ]
        then
            printf "[Updating: %s --> %s]\n" ${TAG} ${LATEST}
            sed \
            --in-place \
            --expression="s|docker.io/${NAMESPACE}/${REPOSITORY}:${TAG}|docker.io/${NAMESPACE}/${REPOSITORY}:${LATEST}|g" \
            .env.test
        else
            printf "[Current]\n"
        fi
    done
}

update_images_dazwilkin
update_images_dockerhub
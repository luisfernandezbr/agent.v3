#!/usr/bin/env bash
#
# This is the installer for the Pinpoint Agent.
#
# For more information, see: https://github.com/pinpt/agent.next
#
# Parts of install script are based on github.com/golang/dep/install.sh, which is licensed under BSD style license: https://github.com/golang/dep/blob/master/LICENSE

# Environment variables:
# - INSTALL_DIRECTORY (optional): defaults to $HOME/.pinpoint-agent
# - PP_INSTALL_RELEASE_TAG (optional): defaults to fetching the latest release
# - PP_INSTALL_OS (optional): use a specific value for OS (mostly for testing)
# - PP_INSTALL_ARCH (optional): use a specific value for ARCH (mostly for testing)

initArch() {
    ARCH=$(uname -m)
    if [ -n "$PP_INSTALL_ARCH" ]; then
        echo "Using PP_INSTALL_ARCH"
        ARCH="$PP_INSTALL_ARCH"
    fi
    case $ARCH in
        amd64) ARCH="amd64";;
        x86_64) ARCH="amd64";;
        #i386) ARCH="386";;
        #ppc64) ARCH="ppc64";;
        #ppc64le) ARCH="ppc64le";;
        #s390x) ARCH="s390x";;
        #armv6*) ARCH="arm";;
        #armv7*) ARCH="arm";;
        #aarch64) ARCH="arm64";;
        *) echo "Architecture ${ARCH} is not supported by this installation script"; exit 1;;
    esac
    echo "ARCH = $ARCH"
}

initOS() {
    OS=$(uname | tr '[:upper:]' '[:lower:]')
    OS_CYGWIN=0
    if [ -n "$PP_INSTALL_OS" ]; then
        echo "Using PP_INSTALL_OS"
        OS="$PP_INSTALL_OS"
    fi
    case "$OS" in
        #darwin) OS='darwin';;
        linux) OS='linux';;
        #freebsd) OS='freebsd';;
        #mingw*) OS='windows';;
        #msys*) OS='windows';;
	#cygwin*)
	#    OS='windows'
	#    OS_CYGWIN=1
	#    ;;
        *) echo "OS ${OS} is not supported by this installation script"; exit 1;;
    esac
    echo "OS = $OS"
}

RELEASES_URL="https://github.com/pinpt/agent/releases"

downloadJSON() {
    url="$2"

    echo "Fetching $url.."
    if test -x "$(command -v curl)"; then
        response=$(curl -s -L -w 'HTTPSTATUS:%{http_code}' -H 'Accept: application/json' "$url")
        body=$(echo "$response" | sed -e 's/HTTPSTATUS\:.*//g')
        code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
    elif test -x "$(command -v wget)"; then
        temp=$(mktemp)
        body=$(wget -q --header='Accept: application/json' -O - --server-response "$url" 2> "$temp")
        code=$(awk '/^  HTTP/{print $2}' < "$temp" | tail -1)
        rm "$temp"
    else
        echo "Neither curl nor wget was available to perform http requests."
        exit 1
    fi
    if [ "$code" != 200 ]; then
        echo "Request failed with code $code"
        exit 1
    fi

    eval "$1='$body'"
}

downloadFile() {
    url="$1"
    destination="$2"

    echo "Fetching $url"
    if test -x "$(command -v curl)"; then
        code=$(curl -s -w '%{http_code}' -L "$url" -o "$destination")
    elif test -x "$(command -v wget)"; then
        code=$(wget -q -O "$destination" --server-response "$url" 2>&1 | awk '/^  HTTP/{print $2}' | tail -1)
    else
        echo "Neither curl nor wget was available to perform http requests."
        exit 1
    fi

    if [ "$code" != 200 ]; then
        echo "Request failed with code $code"
        exit 1
    fi
}

set -e

echo -e "\033[94m
    ____  _                   _       __ 
   / __ \(_)___  ____  ____  (_)___  / /_
  / /_/ / / __ \/ __ \/ __ \/ / __ \/ __/
 / ____/ / / / / /_/ / /_/ / / / / / /_  
/_/   /_/_/ /_/ .___/\____/_/_/ /_/\__/  
             /_/ 
\033[0m"

# identify platform based on uname output
initArch
initOS

# determine install directory if required
if [ -z "$INSTALL_DIRECTORY" ]; then
    INSTALL_DIRECTORY="$HOME/.pinpoint-agent"
fi
mkdir -p "$INSTALL_DIRECTORY"

echo "Will install into $INSTALL_DIRECTORY"

#BINARY="pinpoint-agent-${OS}-${ARCH}"
# TODO: add arch to name in build script
BINARY="pinpoint-agent-${OS}"

if [ "$OS" = "windows" ]; then
    BINARY="$BINARY.exe"
fi

echo $BINARY

RELEASE_TAG="$PP_INSTALL_RELEASE_TAG"

# if PP_INSTALL_RELEASE_TAG was not provided, assume latest
if [ -z "$PP_INSTALL_RELEASE_TAG" ]; then
    downloadJSON LATEST_RELEASE "$RELEASES_URL/latest"
    RELEASE_TAG=$(echo "${LATEST_RELEASE}" | tr -s '\n' ' ' | sed 's/.*"tag_name":"//' | sed 's/".*//' )
fi

echo "Release Tag = $PP_INSTALL_RELEASE_TAG"

# fetch the real release data to make sure it exists before we attempt a download
downloadJSON RELEASE_DATA "$RELEASES_URL/tag/$RELEASE_TAG"

#BINARY_URL="$RELEASES_URL/download/$RELEASE_TAG/$BINARY"
# use s3 url while github repo is private
BINARY_URL="https://pinpoint-agent.s3.amazonaws.com/releases/$RELEASE_TAG/agent/$BINARY"

DOWNLOAD_FILE=$(mktemp)

downloadFile "$BINARY_URL" "$DOWNLOAD_FILE"

echo "Setting executable permissions."
chmod +x "$DOWNLOAD_FILE"

INSTALL_NAME="pinpoint-agent"

if [ "$OS" = "windows" ]; then
    INSTALL_NAME="$INSTALL_NAME.exe"
fi

echo "Moving executable to $INSTALL_DIRECTORY/$INSTALL_NAME"
mv "$DOWNLOAD_FILE" "$INSTALL_DIRECTORY/$INSTALL_NAME"
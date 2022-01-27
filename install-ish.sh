#!/bin/sh
set -e

echo "Installing dependencies..."
apk add curl jq

download_release() {
  echo "Installing $1..."
  LATEST_VERSION=$(curl -sfSL "https://api.github.com/repos/SignTools/$1/releases/latest" | jq -r .tag_name)
  [ "$2" = true ] && FILE_VERSION="${LATEST_VERSION#?}" || FILE_VERSION="$LATEST_VERSION"
  curl -sfSL "https://github.com/SignTools/$1/releases/download/$LATEST_VERSION/$1_${FILE_VERSION}_linux_386" -o "$1"
  chmod +x "$1"
}

download_release "cloudflared" false
download_release "SignTools" true

echo "Installing runner script..."
cat <<EOF >start-signer.sh
#!/bin/sh
set -e

trap 'trap " " SIGTERM; kill 0; wait' SIGINT SIGTERM EXIT INT

echo "Starting service..."
./cloudflared tunnel --url http://localhost:8080 --metrics localhost:51881 --loglevel error &
./SignTools -host localhost -cloudflared-port 51881 &
cat /dev/location > /dev/null &
echo "Press Ctrl+C / ^+C to stop..."
wait -n
EOF
chmod +x start-signer.sh

echo "All done!"
echo "To start the service, use: ./start-signer.sh"

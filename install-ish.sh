#!/bin/sh
set -e

echo "Installing dependencies..."
apk add curl jq unzip

echo "Installing ngrok..."
curl -sL https://bin.equinox.io/c/4VmDzA7iaHb/ngrok-stable-linux-386.zip -o ngrok.zip
unzip -o ngrok.zip
rm ngrok.zip
chmod +x ngrok

echo "Installing service..."
LATEST_VERSION=$(curl -sL "https://api.github.com/repos/signtools/ios-signer-service/releases/latest" | jq -r .tag_name)
curl -sL "https://github.com/SignTools/ios-signer-service/releases/download/$LATEST_VERSION/ios-signer-service_${LATEST_VERSION#?}_linux_386" -o ios-signer-service
chmod +x ios-signer-service

echo "Installing runner script..."
cat <<EOF >start-signer.sh
#!/bin/sh
set -e

trap 'trap " " SIGTERM; kill 0; wait' SIGINT SIGTERM EXIT

echo "Starting service..."
./ngrok http -inspect=false 8080 &
./ios-signer-service -host localhost -ngrok-port 4040 &
cat /dev/location > /dev/null &
sleep 1
echo "Press Ctrl+C / ^+C to stop..."
sleep infinity
EOF
chmod +x start-signer.sh

echo "All done!"
echo "Don't forget to connect your ngrok account if you haven't already:"
echo "./ngrok authtoken YOUR_NGROK_TOKEN"
echo "To start the service, use: ./start-signer.sh"

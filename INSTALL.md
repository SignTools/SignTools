# Installation

There are multiple ways to install this web service:

- On your phone
- On your computer without HTTPS certificate or port forwarding
- On your server with HTTPS and open port 443

But in all cases, you first need a builder.

## 1. Builder

Head over to [ios-signer-ci](https://github.com/SignTools/ios-signer-ci) and follow the instructions. When you are done, you will have made your very own macOS builder.

## 2. Web service configuration

It's easier if you use your personal computer for the initial configuration. This guide assumes you are doing that.

### 2.1. Configuration file

You need to create a configuration file which links the web service to your builder.

1. Download the correct [binary release](https://github.com/SignTools/ios-signer-service/releases) for your computer
2. Run it once - it will exit immediately, saying that it has generated a configuration file
3. In the same directory as the binary, you will find `signer-cfg.yml` - open it with your favorite text editor and configure the settings using the explanations below:

```yml
# the builder you created in the previous section
# you need to configure and enable one
builder:
  github:
    enabled: false
    repo_name: ios-signer-ci
    org_name: YOUR_PROFILE_NAME
    workflow_file_name: sign.yml
    token: YOUR_TOKEN
    ref: master
  semaphore:
    enabled: false
    project_id: YOUR_PROJECT_ID
    org_name: YOUR_ORG_NAME
    token: YOUR_TOKEN
    ref: refs/heads/master
    secret_name: ios-signer
  # use this if you have an unsupported builder
  generic:
    enabled: false
    status_url: http://localhost:1234/status
    trigger_url: http://localhost:1234/trigger
    secrets_url: http://localhost:1234/secrets
    trigger_body: hello
    headers:
      Authroziation: Token YOUR_TOKEN
    attempt_http2: true
# the public address of your server, used to build URLs for the website and builder
# must be valid HTTPS or OTA won't work!
# leave empty if you don't know what this is - you can use ngrok
server_url: https://mywebsite.com
# where to save data like apps and signing profiles
save_dir: data
# apps older than this time will be deleted when a cleanup job is run
cleanup_mins: 10080
# how often does the cleanup job run
cleanup_interval_mins: 30
# protects the web ui with a username and password
# definitely enable if you left "server_url" empty and are using ngrok
basic_auth:
  enable: false
  username: "admin"
  password: "admin"
```

### 2.2. Signing profiles

You need to add at least one code signing profile.

1. Create a new directory named `data` (if you changed `save_dir` above, use that value)
2. Create another directory named `profiles` inside it
3. Create a new directory named `my_profile` inside `profiles`. You can use any name here, this will be the ID of your signing profile
4. Put your signing profile's files inside it
5. Repeat these steps for each signing profile that you want to add

The final directory structure will look like this:

```
data
|____profiles
| |____my_profile                # any unique string that you want
| | |____cert.p12                # the signing certificate
| | |____pass.txt                # the signing certificate's password
| | |____name.txt                # a name to show in the web interface
| | |____prov.mobileprovision    # the signing provisioning profile
| |____my_other_profile
| | |____...
```

That's all the initial configuration! To recap, you now have the following configuration files:

- `data` directory (or whatever you set in `save_dir`)
- `signer-cfg.yml` file

## 3. Web service installation

You can install the web service on your computer, or you can install it on your phone. The device that you choose will have to be connected to the internet in order for anybody to use the service.

### 3.1. Self-hosting on phone

#### 3.1.1. Preparing

1. Register for [ngrok](https://ngrok.com/)
1. Get the [iSH](https://ish.app/) app on your phone
1. Move the configuration files from sections `2.1.` and `2.2.` to your phone. You can use any method, like [iTunes](https://www.apple.com/us/itunes/) or [iMazing](https://imazing.com/). It doesn't matter where you put the files as long as you can access them from the Files app on your phone
1. Open the Files app on your phone
1. In the top-right corner, click on the three dots and select `Edit`
1. Enable (toggle) the `iSH` entry under `Locations`
1. Move the files you just copied from your computer to the `iSH` location you just enabled, inside the directory `root`
1. Open the `iSH` app on your phone. It may take some time before text shows up.
1. Type `ls` and press enter. If you did everything correctly, you should see the names of the files you just moved in

#### 3.1.2. Installing

1. Type the following command and press enter:
   ```bash
   curl -sL git.io/ios-signer-ish | sh
   ```
2. If you haven't already, connect your ngrok account:
   ```bash
   ./ngrok authtoken YOUR_NGROK_TOKEN
   ```

#### 3.1.3. Running

1. Type the following command and press enter:
   ```bash
   ./start-service.sh
   ```
2. When the service finishes loading, look for a line similar to this:
   ```log
   2021/03/08 20:50:02 ngrok public URL: https://2a25cbf1a2d4.ngrok.io
   ```
   `https://xxxxxxxxxxxx.ngrok.io` is the public URL of your service. That's what you want to open in your browser. Congratulations!

#### 3.1.4. Updating

Just repeat the `Installing` section.

### 3.2. Self-hosting on computer

#### 3.2.1. Installing

You have two options:

**Raw**

1. Download the correct [binary release](https://github.com/SignTools/ios-signer-service/releases)
2. Move the configuration files from sections `2.1.` and `2.2.` to the same directory as the binary you just downloaded

**Docker**

1. Use the official [Docker image](https://hub.docker.com/r/signtools/ios-signer-service)
2. Move and [mount](https://docs.docker.com/storage/volumes/) the configuration files from sections `2.1.` and `2.2.`:
   - `./signer-cfg.yml:/signer-cfg.yml`
   - `./data:/data` (or whatever you set in `save_dir`)

#### 3.2.2. Running

Default arguments:

- The listening port is 8080. You can change this with the argument `-port 1234`
- The listening host is all (0.0.0.0). You can change this with the argument `-host 1.2.3.4`
- For more, use `-help`

The web service cannot work by itself. You have two options:

**Reverse proxy** - secure and reliable, but harder to set up

- Requires publicly accessible port 443 (HTTPS)
- Requires domain with valid certificate
- Requires manual configuration of reverse proxy with your own authentication.
- Don't protect the following endpoints:
  ```
  /apps/:id/
  /jobs
  /jobs/:id
  ```
  where `:id` is a wildcard parameter.

**ngrok** - less secure (unless you trust ngrok), but quick and easy to set up

1. Register for [ngrok](https://ngrok.com/)
2. Install ngrok and connect your account as instructed on the dashboard
3. **Every time before** starting your service, run the following command:
   ```bash
   ngrok http -inspect=false 8080
   ```
4. Then start your service with the following arguments:
   ```bash
   ios-signer-service -ngrok-port 4040 -host localhost
   ```
5. When the service finishes loading, look for a line similar to this:
   ```log
   2021/03/08 20:50:02 ngrok public URL: https://2a25cbf1a2d4.ngrok.io
   ```
   `https://xxxxxxxxxxxx.ngrok.io` is the public URL of your service. That's what you want to open in your browser. Congratulations!

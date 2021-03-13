# Installation

There are multiple ways to install this web service:

- On your phone
- On your computer, without HTTPS certificate or port forwarding
- On your server, with HTTPS and open port 443

But in all cases, you first need a builder.

## 1. Builder

Head over to [ios-signer-ci](https://github.com/SignTools/ios-signer-ci) and follow the instructions. When you are done, you will have made your very own macOS builder.

## 2. Web service configuration

It's easier if you use your personal computer for the initial configuration. This guide assumes you are doing that.

### 2.1. Configuration file

You need to create a configuration file which links the web service to your builder.

1. Download the correct [binary release](https://github.com/SignTools/ios-signer-service/releases) for your computer
2. Run it once - it will exit immediately, saying that it has generated a configuration file
3. In the same folder as the binary, you will find a new file `signer-cfg.yml` - open it with your favorite text editor and configure the settings using the explanations below:

```yml
# the builder you created in the previous section
# you need to configure and enable one
builder:
# Set enabled to 'true' if you've set up GitHub Actions
  github:
    enabled: false
    repo_name: ios-signer-ci # This needs to match the name of your cloned ios-signer-ci repository.
    org_name: YOUR_PROFILE_NAME # Put your GitHub profile name here, or organization if you are using one.
    workflow_file_name: sign.yml
    token: YOUR_GITHUB_TOKEN # Insert the Personal Access Token from GitHub you created earlier.
    ref: master
# Set enabled to 'true' if you've set up Semaphore CI
  semaphore:
    enabled: false
    project_id: YOUR_PROJECT_ID # This is the project ID you've gotten from Semaphore ID
    org_name: YOUR_ORG_NAME # Put your Semaphore profile name here, or organization if you are using one.
    token: YOUR_SEMAPHORE_TOKEN # Insert the Personal Access Token from Semaphore CI you created earlier.
    ref: refs/heads/master
    secret_name: ios-signer
  # Set enabled to 'true' if you use an unsupported builder.
  generic:
    enabled: false
    status_url: http://localhost:1234/status
    trigger_url: http://localhost:1234/trigger
    secrets_url: http://localhost:1234/secrets
    trigger_body: hello
    headers:
      Authorization: Token YOUR_CUSTOM_TOKEN
    attempt_http2: true
# the public address of your server, used to build URLs for the website and builder
# must be valid HTTPS or OTA won't work!
# leave it empty if you don't know what it is - use ngrok instead.
server_url: https://mywebsite.com
# where to save data like apps and signing profiles
save_dir: data
# apps older than this time will be deleted when a cleanup job is run
cleanup_mins: 10080
# how often does the cleanup job run
cleanup_interval_mins: 30
# apps older than this time will be marked as failed
# this should also match the job timeout in the builder
sign_timeout_mins: 10
# this protects the web ui with a username and password
# definitely enable it if you left "server_url" empty and are using ngrok
basic_auth:
  enable: false
  username: "admin"
  password: "admin" # Please change the password!
```

### 2.2. Signing profiles

You need to add at least one code signing profile.

1. Create a new folder named `data` (if you changed `save_dir` above, use that value)
2. Create another folder named `profiles` inside of it
3. Create a new folder named `my_profile` inside of `profiles`. You can use any profile name here, this will be the ID of your signing profile
4. Put the signing related files into here. [Learn how to get your certificate files](FAQ.md).
5. Repeat these steps for each signing profile that you want to add

There are two types of signing profiles, each with different requirements. If you can, use a "certificate + provisioning profile" - it will save you from a lot of limitations. For more information and help, check out the [FAQ page](FAQ.md). The folder structures are as follows:

> ### Note :warning:: You **need** to name your files the same as they are in the illustrated folder structures. (e.g Certificates.p12 to cert.p12)

- Certificate + provisioning profile folder structure (`cert.p12`, `cert_pass.txt`, `prov.mobileprovision`)

  ```yaml
  data
  |____profiles
  | |____my_profile                # any unique string that you want
  | | |____cert.p12                # the signing certificate
  | | |____cert_pass.txt           # the signing certificate's password
  | | |____name.txt                # a name to show in the web interface
  | | |____prov.mobileprovision    # the signing provisioning profile
  | |____my_other_profile
  | | |____...
  ```

- Free developer account folder structure (`cert.p12`, `cert_pass.txt`, `account_name.txt`, `account_pass.txt`)

  Make sure to read the [FAQ page](FAQ.md) to understand the limitations of using a free developer account!

  ```yaml
  data
  |____profiles
  | |____my_profile                # Or what you named your profile
  | | |____cert.p12                # the signing certificate
  | | |____cert_pass.txt           # the signing certificate's password
  | | |____name.txt                # a name to show in the web interface
  | | |____account_name.txt        # the developer account's name (email)
  | | |____account_pass.txt        # the developer account's password
  | |____my_other_profile
  | | |____...
  ```

That's all the initial configuration! To recap, you now have the following configuration files:

- `data` folder (or whatever you named it in `save_dir` in the config)
- `signer-cfg.yml` file

## 3. Web service installation

You can install the web service on your computer a server, or your phone. The device that you choose will have to be connected to the internet in order for anybody to use the service.

### 3.1. Self-hosting on phone

#### 3.1.1. Preparing

1. Register for [ngrok](https://ngrok.com/). The setup page, other than the download, presents a quick set-up guide with some commands. Note down the `ngrok authtoken` one.
2. Get the [iSH](https://ish.app/) app on your phone and open it once first.
3. Move the configuration files you made in sections `2.1.` and `2.2.` in this guide to your phone. You can use any method, like [iTunes](https://www.apple.com/us/itunes/) or [iMazing](https://imazing.com/). It doesn't matter where you put the files as long as you can access them from the Files app on your phone
4. Open the Files app on your phone.
5. In the top-right corner, click on the three dots and select `Edit`.
6. Enable (toggle) the `iSH` entry under `Locations`
7. Move the files you just copied from your computer to the `iSH` location you just enabled, inside the folder `root`
8. Open the `iSH` app again.
9. Type `ls` and press enter. If you did everything correctly, you should see the names of the files you just moved in
10. Type `apk add curl` and press enter

#### 3.1.2. Installing

1. Type the following command and press enter:
   ```bash
   curl -sL git.io/ios-signer-ish | sh
   ```
2. If you haven't already, connect your ngrok account as instructed on the download page:
   ```bash
   ./ngrok authtoken YOUR_NGROK_TOKEN
   ```

#### 3.1.3. Running

1. Type the following command and press enter:
   ```bash
   ./start-signer.sh
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

**Normal**

1. Download the correct [binary release](https://github.com/SignTools/ios-signer-service/releases) (if this is a different computer than you used before)
2. Move the configuration files from sections `2.1.` and `2.2.` to the same folder as the binary you just downloaded

**Docker**

1. Use the official [Docker image](https://hub.docker.com/r/signtools/ios-signer-service)
2. Move and [mount](https://docs.docker.com/storage/volumes/) the configuration files from sections `2.1.` and `2.2.`:
   - `./signer-cfg.yml:/signer-cfg.yml`
   - `./data:/data` (or whatever you set in `save_dir`)

#### 3.2.2. Running

For overview, these are the default arguments that will be used:

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
2. Download ngrok and connect your account as instructed on the download page
3. **Every time before** starting your service, execute the following command and keep it running:
   ```bash
   ngrok http -inspect=false 8080
   ```
4. Then start your service with the following command:
   ```bash
   ios-signer-service -ngrok-port 4040 -host localhost
   ```
5. When the service finishes loading, look for a line similar to this:
   ```log
   2021/03/08 20:50:02 ngrok public URL: https://2a25cbf1a2d4.ngrok.io
   ```
   `https://xxxxxxxxxxxx.ngrok.io` is the public URL of your service. That's what you want to open in your browser. Congratulations!

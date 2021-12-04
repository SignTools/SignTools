# Advanced Installation

Before you begin, it is recommended to understand exactly how this project works. Knowing what is happening at each point will help you troubleshoot any issues far better. Check out the [How does this all work?](DETAILS.md) page.

## Video tutorial

For a video tutorial, [click here](https://youtu.be/Tcco-bES1-M). **You still need this written guide** - the video is somewhat outdated, and it does not cover everything.

## Table of Contents

- [Advanced Installation](#advanced-installation)
  - [Video tutorial](#video-tutorial)
  - [Table of Contents](#table-of-contents)
  - [1. Builder](#1-builder)
  - [2. Web service configuration](#2-web-service-configuration)
    - [2.1. Configuration file](#21-configuration-file)
    - [2.2. Signing profile](#22-signing-profile)
  - [3. Web service installation](#3-web-service-installation)
    - [3.1. Self-hosting on computer or server](#31-self-hosting-on-computer-or-server)
      - [3.1.1. Installing](#311-installing)
      - [3.1.2. Running](#312-running)
    - [3.2. Self-hosting on phone](#32-self-hosting-on-phone)
      - [3.2.1. Preparing](#321-preparing)
      - [3.2.2. Installing](#322-installing)
      - [3.2.3. Running](#323-running)
      - [3.2.4. Updating](#324-updating)

## 1. Builder

You can create a builder in one of two ways:

- **Use a Continuous Integration (CI) service** such as GitHub Actions or Semaphore CI. This method is the easiest, fastest, and most recommended way to make a builder. Head over to [SignTools-CI](https://github.com/SignTools/SignTools-CI) and follow the instructions.

- **Use your own Mac machine**. This method is only recommended if you already have a server Mac, you are somewhat experienced in server management, and you would like to host your truly own builder. Go to [SignTools-Builder](https://github.com/SignTools/SignTools-Builder) for instructions.

Only one builder is necessary, but you can have more if needed. Once done, proceed below.

## 2. Web service configuration

It's easier if you use your personal computer for the initial configuration. This guide assumes you are doing that.

### 2.1. Configuration file

You need to create a configuration file which links the web service to your builder.

1. Download the correct [binary release](https://github.com/SignTools/SignTools/releases) for your computer
2. Run it once - it will exit immediately, saying that it has generated a configuration file
3. In the same folder as the binary, you will find a new file `signer-cfg.yml` - open it with your favorite text editor and configure the settings using the explanations below. The lines that start with a hashtag `#` are comments, you do not need to touch them.

> :warning: **Don't forget to set "`enable: true`" on the builder that you are configuring!**

```yml
# configure the builder(s) you created in the previous section
builder:
  # GitHub Actions
  github:
    enable: false
    # the name you gave your builder repository
    repo_name: SignTools-CI
    # your GitHub profile/organization name
    org_name: YOUR_ORG_NAME
    workflow_file_name: sign.yml
    # your GitHub personal access token that you created with the builder
    token: YOUR_GITHUB_TOKEN
    ref: master
  # Semaphore CI
  semaphore:
    enable: false
    # the name you gave to your Semaphore CI project
    project_name: YOUR_PROJECT_NAME
    # your Semaphore CI profile/organization name
    org_name: YOUR_ORG_NAME
    # your Semaphore CI token that you got when creating the builder
    token: YOUR_SEMAPHORE_TOKEN
    ref: refs/heads/master
    secret_name: ios-signer
  # your own self-hosted Mac builder
  selfhosted:
    enable: false
    # the url of your builder
    url: http://192.168.1.133:8090
    # the auth key you used when you started the builder
    key: SOME_SECRET_KEY
# the base url used by the builder to reach this server
# leave empty if using a tunnel provider, it will set this automatically
server_url: https://mywebsite.com
# whether to redirect all http requests to https
redirect_https: false
# where to save data like apps and signing profiles
save_dir: data
# how often does the cleanup job run
cleanup_interval_mins: 1
# apps that have been processing for more than this time will be marked as failed
sign_timeout_mins: 15
# this protects the web ui with a username and password
# definitely enable it if you are using a tunnel provider
basic_auth:
  enable: false
  username: "admin"
  # don't forget to change the password
  password: "admin"
```

### 2.2. Signing profile

You need a signing profile to be able to sign apps. A signing profile is simply a collection of files and credentials that Apple provides to developers so they can sign apps.

There are two types of signing profiles:

- **Developer account**

  This method works for both free and paid developer accounts. You only need your Apple account's name and password. You will likely be prompted for a 6-digit code every time you sign an app, which you can submit on the service's web page. This method will be able to use most entitlements, resulting in working app extensions and iCloud synchronization. There are no restrictions if you have a paid account. If you have a free account, make sure you read and understand the limitations in the [FAQ](FAQ.md#free-developer-account-limitations) page.

- **Custom provisioning profile**

  If you have a provisioning profile with a `.mobileprovision` extension, you can use this method as well. There is no 6-digit code, so signing will be faster than a developer account. However, based on the type of your provisioning profile, different entitlements and features may not work on your signed apps. For the differences, check the [FAQ](FAQ.md#what-kind-of-certificatesprovisioning-profiles-are-supported) page.

Additionally, you will also need a certificate archive with a `.p12` extension. It must contain at least one certificate and at least one private key. You can either use an `Apple Development` certificate, or both `Apple Development` and `Apple Distribution` if you want to use production entitlements. For the differences, check the [FAQ](FAQ.md#what-kind-of-certificatesprovisioning-profiles-are-supported) page.

If you are using a custom provisioning profile, you likely received a certificate archive along with it - use that. If you have a developer account, you can create one from the [developer portal](https://developer.apple.com/account/resources/certificates/list). Otherwise, follow the instructions below:

- **macOS**

  Install [Xcode](https://developer.apple.com/xcode/) and open the `Account Preferences` (A). Sign into your account using the plus button. Select your account and click on `Manage Certificates...`. In the new window (B), click the plus button and then `Apple Development`. Click `Done`. Now open the `Keychain` app (C). There you will find your certificate and private key. Select them by holding `Command`, then right-click and select `Export 2 items...`. This will export you the `.p12` file you need.

  <table>
  <tr>
      <th>A</th>
      <th>B</th>
      <th>C</th>
  </tr>
  <tr>
      <td><img src="img/6.png"/></td>
      <td><img src="img/7.png"/></td>
      <td><img src="img/5.png"/></td>
  </tr>
  </table>

- **Windows**

  There is no official way to do this. However, you can use [altserver-cert-dumper](https://github.com/SignTools/altserver-cert-dumper) with [AltStore](https://altstore.io/) as a workaround. Note that you are doing so at your own risk.

Once you have your signing profile, you need to create the correct folders for the service to read it:

1. Create a new folder named `data` (if you changed `save_dir` in the config above, use that)
2. Create another folder named `profiles` inside of it
3. Create a new folder named `my_profile` inside of `profiles`. You can use any profile name here, this will be the ID of your signing profile
4. Put the signing related files inside here. Read ahead to see what they should be named
5. Repeat the steps above for each signing profile that you want to add

> :warning: **You need to match the files names exactly as they are shown below. For an example, your certificate archive must be named exactly `cert.p12`. Be aware that Windows may hide the extensions by default.**

- **Developer account**

  ```python
  data
  |____profiles
  | |____my_profile                # Or what you named your profile
  | | |____cert.p12                # the signing certificate archive
  | | |____cert_pass.txt           # the signing certificate archive's password
  | | |____name.txt                # a name to show in the web interface
  | | |____account_name.txt        # the developer account's name (email)
  | | |____account_pass.txt        # the developer account's password
  | |____my_other_profile
  | | |____...
  ```

- **Custom provisioning profile**

  ```python
  data
  |____profiles
  | |____my_profile                # any unique string that you want
  | | |____cert.p12                # the signing certificate archive
  | | |____cert_pass.txt           # the signing certificate archive's password
  | | |____name.txt                # a name to show in the web interface
  | | |____prov.mobileprovision    # the signing provisioning profile
  | |____my_other_profile
  | | |____...
  ```

That's all the initial configuration! To recap, you now have the following configuration files:

- `data` folder (or whatever you named it in `save_dir` in the config)
- `signer-cfg.yml` file

## 3. Web service installation

### 3.1. Self-hosting on computer or server

#### 3.1.1. Installing

You have two options:

**Normal**

1. Download the correct [binary release](https://github.com/SignTools/SignTools/releases) (if this is a different computer than you used before)
2. Move the configuration files you made in sections `2.1.` and `2.2.` of this guide to the same folder as the binary you just downloaded

**Docker**

1. Use the official [Docker image](https://hub.docker.com/r/signtools/SignTools)
2. Move and [mount](https://docs.docker.com/storage/volumes/) the configuration files from sections `2.1.` and `2.2.`:
   - `./signer-cfg.yml:/signer-cfg.yml`
   - `./data:/data` (or whatever you set in `save_dir`)

#### 3.1.2. Running

For reference, these are the default arguments that will be used:

- The listening port is 8080. You can change this with the argument `-port 1234`
- The listening host is all (0.0.0.0). You can change this with the argument `-host 1.2.3.4`
- For more, use `-help`

The web service cannot work by itself. You have two options:

**Reverse proxy** - secure, fast, reliable, but harder to set up

- Requires publicly accessible port 443 (HTTPS)
- Requires domain with valid HTTPS certificate
- Requires manual configuration of reverse proxy with your own authentication
- Don't protect the following endpoints:
  ```
  /apps/:id/
  /jobs
  /jobs/:id
  /jobs/:id/tus/
  /files/:id
  ```
  where `:id` is a wildcard parameter.

**Tunnel provider** - less secure, slower, but quick and easy to set up

[ngrok](https://ngrok.com/) and [Cloudflare Argo](https://blog.cloudflare.com/a-free-argo-tunnel-for-your-next-project/#how-can-i-use-the-free-version) are supported as tunnel providers. The latter will be demonstrated in this guide. Run the signer service with `-help` to see alternative details.

1. Download the correct [cloudflared](https://github.com/cloudflare/cloudflared/releases/latest) binary for your computer.
2. **Every time before** starting your service, execute the following command and keep it running:
   ```bash
   cloudflared tunnel -url http://localhost:8080 -metrics localhost:51881
   ```
3. Then start your service with the following command:
   ```bash
   SignTools -cloudflared-port 51881
   ```
4. When the service finishes loading, look for a line similar to this:
   ```log
   11:51PM INF using server url url=https://aids-woman-zum-summer.trycloudflare.com
   ```
   `https://xxxxxxxxxxxx.trycloudflare.com` is the public address of your service. That's what you want to open in your browser. If you want faster transfer speeds, you can also use the LAN or localhost IP address. Congratulations!

### 3.2. Self-hosting on phone

It is highly discouraged to use this method unless you absolutely need to. There are a lot of limitations and weird quirks, and signing will be _very_ slow.

#### 3.2.1. Preparing

1. Get the [iSH](https://ish.app/) app on your phone. Open the app, and when text appears, close it.
2. Move the configuration files you made in sections `2.1.` and `2.2.` of this guide to your phone. You can use any method, like [iTunes](https://www.apple.com/us/itunes/) or [iMazing](https://imazing.com/). It doesn't matter where you put the files as long as you can access them from the Files app on your phone.
3. Open the Files app on your phone.
4. Swipe to the right until you go to the `Browse` screen.
5. In the top-right corner, click on the three dots and select `Edit`.
6. Enable (toggle) the `iSH` entry under `Locations`.
7. Move the files you just copied from your computer to the `iSH` location you just enabled, inside the folder `root`.
8. Open the `iSH` app again.
9. Type `ls` and press enter. If you did everything correctly, you should see the names of the files you just moved in.
10. Type `apk add curl` and press enter.

#### 3.2.2. Installing

1. Type the following command and press enter:
   ```bash
   curl -sL git.io/ios-signer-ish | sh
   ```

#### 3.2.3. Running

1. Type the following command and press enter:

   ```bash
   ./start-signer.sh
   ```

   > :warning: **When iOS asks you to grant location permission to iSH, click "Always Allow". The location data is not used for anything, but the permission allows the service to keep running in the background if you minimize iSH or lock your phone.**

2. When the service finishes loading, look for a line similar to this:

   ```log
   11:51PM INF using server url url=https://aids-woman-zum-summer.trycloudflare.com
   ```

   `https://xxxxxxxxxxxx.trycloudflare.com` is the public address of your service. That's what you want to open in your browser. If you want faster transfer speeds, you can also use the LAN or localhost IP address. Congratulations!

   Due to Apple's strict background process policy, iSH will get killed if it uses more than "80% cpu over 60 seconds". This can break any part of the service. To make sure it doesn't happen:

   > :warning: **Use Safari browser. Whenever you are waiting for something to complete, such as app upload, signing, or installation, open iSH in the foreground. Safari will continue in the background.**

   Watch the logs to know when the operation is complete. You can periodically switch between Safari and iSH to check as well, just don't leave iSH in the background for more than 30 seconds.

#### 3.2.4. Updating

Just repeat the `Installing` section.

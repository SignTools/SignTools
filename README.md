# iOS Signer Service

## Introduction

There are many reasons to install apps outside Apple's App Store. Unfortunately, this process is severely hindered by Apple and unrealistic for the average user. You need a better way to get things done.

Introducing `ios-signer-service` - a self-hosted, cross-platform service to sign and install iOS apps, all **without a computer**.

What's the catch? The workflow uses a separate macOS machine, called a "builder", to perform the signing. This project is only a web service which controls the builder, along with a nice web interface for you to upload unsigned apps and download signed apps. However, **you don't need to own a Mac** - the builder can be any free CI (Continuous Integration) provider or your own machine. The web service can be hosted on any computer or **even your phone**.

More information and examples can be found in the installation section.

## Disclaimer

This project is self-hosted; there is no public service. It does not provide any alternative catalog of apps. It does not give you free signing certificates, or circumvent any protective measures - **you must have a valid signing certificate and provisioning profile**. This project does not provide, promote, or support any form of piracy. This project is aimed solely at people who want to install homebrew apps on their device, much like the popular [AltStore](https://github.com/rileytestut/AltStore).

## Features

- No jailbreak required
- All iOS versions supported
- No computer required at all
- Minimalistic, mobile-friendly web interface
- Upload unsigned apps, download signed apps
- Install signed apps from the website straight to your iOS device via [OTA](https://medium.com/@adrianstanecki/distributing-and-installing-non-market-ipa-application-over-the-air-ota-2e65f5ea4a46)
- Choose from multiple signing profiles
- Configure various properties of the signing process
- Periodic old file cleanup

## Screenshots

<table>
<tr>
    <th>Mobile</th>
    <th>Desktop</th>
</tr>
<tr>
    <td>
        <img src="img/3.png"/>
        <img src="img/4.png"/>
    </td>
    <td>
        <img src="img/1.png"/>
        <img src="img/2.png"/>
    </td>
</tr>
</table>

## Installation

There are multiple ways to install this service:

- On your phone
- On your computer without HTTPS certificate or port forwarding
- On your server with HTTPS and open port 443

But more on that later. In all cases, you first need a builder.

### 1. Builder

`ios-signer-service` offloads the signing process to a dedicated macOS builder. This step is necessary because signing is only officially supported on a macOS system. While third-party cross-platform alternatives exist, they are in gray legal grounds and constantly break due to Apple changes.

As mentioned before, you don't need to own a Mac to get a builder. In fact, you don't even need to pay anything. Official support is included for free CI providers such as [GitHub Actions](https://docs.github.com/en/actions) and [Semaphore CI](https://semaphoreci.com/). To get started, head over to [ios-signer-ci](https://github.com/SignTools/ios-signer-ci). Fork the repo and follow its README. At the end, you will have made your very own macOS builder. Congratulations!

You can always use another CI provider, or even your own machine, given they implement the generic builder requirements. Read on for more information.

### 2. Web service

#### 2.1. Configuration file

You need to configure the web service before it will work - the defaults won't be enough!

No matter where you are hosting the service, it easier to use your personal computer for this initial setup. You can replicate the steps entirely on a server or even your phone, but you will be on your own.

Grab the appropriate [binary release](https://github.com/SignTools/ios-signer-service/releases) for your computer and run it once. It will exit immediately, saying that it has generated a configuration file. In the same directory as the binary, you will find `signer-cfg.yml`. Open it with your favorite text editor and configure the settings using the explanations below:

```yml
# the builder you created in the previous section
# if you enable more than one, the first will be used
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

#### 2.2. Signing profiles

You need to create the `save_dir` directory from above ("data" by default), with another directory "profiles" inside it. There you need to add at least one code signing profile. The structure is as follows:

```
data
|____profiles
| |____PROFILE_ID              # any unique string that you want
| | |____cert.p12              # the signing certificate
| | |____pass.txt              # the signing certificate's password
| | |____name.txt              # a name to show in the web interface
| | |____prov.mobileprovision  # the signing provisioning profile
| |____OTHER_PROFILE_ID
| | |____...
```

That's all the setup! You now have two configuration files:

- data directory ("data" or what you set in `save_dir`)
- configuration file (`signer-cfg.yml`)

If you see "configuration files" mentioned in the next sections, it means these files.

#### 2.3. Self-hosting on phone

Using a file manager like iTunes or [iMazing](https://imazing.com/) on your computer, copy the configuration files anywhere on your phone. It doesn't matter where you put them as long as you can access the files from the Files app on your phone later on.

Register for [ngrok](https://ngrok.com/). This is a free service that will allow anybody to connect to your service over HTTPS without actually needing a certificate or any port forwarding.

Get the [iSH](https://ish.app/) app on your phone. This is a brilliant x86 terminal emulator that will let you run ngrok and the web service.

Open the Files app on your phone. In the top-right corner, you should see three dots. Click on them, then select `Edit`. You will notice a new disabled (untoggled) item under `Locations` called `iSH` - enable (toggle) it. In the top-right corner again, click `Done`. This will permanently add the iSH filesystem to your Files app. For pictures, check the iSH [wiki page](https://github.com/ish-app/ish/wiki/View-iSH-files-in-Files-App).

In the Files app again, browse the files you just copied from your computer. Move them to the newly enabled iSH location, under the directory `root`.

Open the iSH app. Type `ls` and press enter. If you did everything correctly, you should see the names of the files you just moved in. Now, type the following command and press enter:

```bash
curl -sL https://raw.githubusercontent.com/SignTools/ios-signer-service/master/install-ish.sh | sh
```

This will download and install ngrok and `ios-signer-service`. Don't forget to connect your ngrok account if you haven't already:

```bash
./ngrok authtoken YOUR_NGROK_TOKEN
```

To start the service, type `./start-signer.sh` and press enter. When the service finishes loading, look for a line similar to this:

```log
2021/03/08 20:50:02 ngrok public URL: https://2a25cbf1a2d4.ngrok.io
```

`https://*.ngrok.io` is the public URL of your service. You can now minimize iSH and open the link in your browser. Congratulations!

Every next time that you want to start the service, just run `./start-signer.sh` again. If you want to update, run the install command again.

#### 2.4. Self-hosting on computer

Just like before, you can use the appropriate [binary release](https://github.com/SignTools/ios-signer-service/releases). Make sure to copy the configuration files in the same directory as the binary.

You can also use the official [Docker image](https://hub.docker.com/r/signtools/ios-signer-service), but make sure to configure it properly:

- Mount the configuration file. Its location in the container is just under the root directory: `/signer-cfg.yml`.
- Mount the data directory. By default, the location in the container is: `/data`.

The service will listen on port 8080. You can override this by running the service with an argument `-port 1234`, where 1234 is your desired port. You can see a description of these arguments and more via `-help`.

`ios-signer-service` is not designed to run by itself - it does not offer encryption (HTTPS) or global authentication. This is a huge security issue, and OTA installations will not work! Instead, you have two options:

- **Reverse proxy**

  If you are looking for the most secure and reliable setup, then this is what you want. Run a reverse proxy like nginx, which wraps the service with HTTPS and authentication. You will need a valid HTTPS certificate (self-signed won't work with OTA due to Apple restriction), which means that you will also need a domain.

  You must leave a few endpoints non-authenticated, as they are used by OTA and the builder. Don't worry, they are secured by long ids and/or the builder key:

  ```
  /apps/:id/
  /jobs
  /jobs/:id
  ```

  where `:id` is a wildcard parameter.

- **ngrok**

  If you are just testing or can't afford the option above, you can also use [ngrok](https://ngrok.com). This is a free service that will allow anybody to connect to your service over HTTPS without actually needing a certificate or any port forwarding.

  To use ngrok, register on their website, then install the program as instructed on their page. Make sure to connect your ngrok account. **Every time before** starting your service, run the following command:

  ```bash
  ngrok http -inspect=false 8080
  ```

  Then start your service with the following arguments:

  ```bash
  ios-signer-service -ngrok-port 4040 -host localhost
  ```

  `-ngrok-port 4040` will allow the service to automatically parse ngrok's current HTTPS URL and set it in the builder. `-host localhost` will restrict the unencrypted connections to your computer only. ngrok will take care of the rest.

  When the service finishes loading, look for a line similar to this:

  ```log
  2021/03/08 20:50:02 ngrok public URL: https://2a25cbf1a2d4.ngrok.io
  ```

  `https://*.ngrok.io` is the public URL of your service. You can now open it in your browser. Congratulations!

## Frequently Asked Questions (F.A.Q.)

- ### How do you export the certificate and key?

  On your Mac, open the `Keychain` app. There you will find your certificate (1) and private key (2). Select them by holding `Command`, then right-click (3) and select `Export 2 items...` (4). This will export you the `.p12` file you need.

  ![](img/5.png)

- ### How can I debug a failing builder?

  Edit the `sign.sh` file in your builder's repo and remove the output suppression from the failing line. Usually this will be the `xresign.sh` call, so:

  ```bash
  ./xresign.sh ...  >/dev/null 2>&1
  ```

  Becomes:

  ```bash
  ./xresign.sh ...
  ```

  Next time you run a build, the logs will give you full details that you can use to resolve your issue. The reason that the output suppression is there in the first place is to prevent leaks of potentially sensitive information about your certificates and apps.

- ### What kind of certificates/provisioning profiles are supported?

  Technically, everything is supported as long as your iOS device trusts it. This includes free signing profiles, but of course, they expire after a week. The only major difference between signing profiles is based on the provisioning profile's `application-identifier`. There are two types:

  - Wildcard, with app id = `TEAM_ID.*`

    - Can properly sign any app (`TEAM_ID.app1`, `TEAM_ID.app2`, ...)
    - Can't use special entitlements such as app groups (Apple restriction)

  - Explicit, with app id = `TEAM_ID.app1`
    - Can properly sign only one app (`TEAM_ID.app1`)
    - Can use any entitlement as long as it's in the provisioning profile
    - If you properly sign multiple apps with the same profile, only one of the apps can be installed on your device at a time. This is because their bundle ids will be identical and the apps will replace each other.
    - It is possible to improperly sign apps with an explicit profile by keeping their original bundle ids even if they don't match the profile's app id. For an example, with an app id `TEAM_ID.app1`, you could sign the apps `TEAM_ID.app2` and `TEAM_ID.app3`. This way, you can have multiple apps installed at the same time, and they will run, but all of their entitlements will be broken, including file importing.

- ### App runs, but malfunctions due to invalid signing/entitlements

  First, make sure you are signing the app correctly and not breaking the entitlements. Read the section just above.

  If that doesn't help, you need to figure out what entitlements the app requires. unc0ver 6.0.2 and DolphiniOS emulator need the app debugging (`get-task-allow`) entitlement. Make sure you are using a signing profile with `get-task-allow=true` in its provisioning profile. Also, when you upload such an app to this service, make sure to tick the `Enable app debugging` option. Since this is a potential security issue, it will be disabled by default unless you tick the box.

- ### "This app cannot be installed because its integrity could not be verified."

  This error means that the signing process went terribly wrong. To debug the problem, install [libimobiledevice](https://libimobiledevice.org/) (for Windows: [imobiledevice-net](https://github.com/libimobiledevice-win32/imobiledevice-net)). Download the problematic signed app from your service to your computer, and then attempt to install it on your iOS device:

  ```bash
  ideviceinstaller -i app.ipa
  ```

  You can also use `-u YOUR_UDID -n` to run this command over the network. When the installation finishes, you should see a more detailed error. Please create an issue here on GitHub and upload the unsigned app along with the detailed error from above so this can be fixed.

## License

This project and all of its unlicensed dependencies under the [SignTools](https://github.com/SignTools) organization are licensed under AGPL-3.0. A copy of the license can be found [here](LICENSE). Raise an issue if you are interested in exclusive licensing.

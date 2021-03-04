# iOS Signer Service

> A self-hosted, cross-platform service to sign iOS apps using any CI as a builder

## Introduction

There are many reasons to install apps outside the App Store. Unfortunately, this process is severely hindered by Apple and unrealistic for the average user. You need a better way to get things done.

Introducing `ios-signer-service` - a self-hosted, cross-platform service to sign iOS apps and install them on your device, all without a computer.

The setup consists of two parts:

- This web service, which runs on a server with any operating system/architecture, and exposes a website where you can upload apps for signing. The website is the only place a user interacts with.
- A macOS builder server, which the web service uses to perform the actual signing. The builder requirements are minimal, so any API-enabled Continuous Integration (CI) service, such as GitHub Actions, can be used.

More information and examples can be found in the installation section.

## Disclaimer

This project is self-hosted; there is no public service. It does not provide any alternative catalog of apps. It does not give you free signing certificates, or circumvent any protective measures - you must have a valid **signing certificate** and **provisioning profile**. This project does not provide, promote, or support any form of piracy. This project is aimed solely at people who want to install homebrew apps on their device, much like the popular [AltStore](https://github.com/rileytestut/AltStore).

## Features

- No jailbreak required
- All iOS versions supported
- No computer required (apart from server to host the service)
- Works with most CI providers, even for free
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

### Requirements

- Web service server
  - All major operating systems and architectures are supported
  - Tested on a Raspberry Pi
- Builder server, such as a CI like GitHub Actions, that:
  - Runs macOS
  - Supports workflow triggers via API
- Valid code signing profile:
  - Certificate with key (`.p12` file)
  - Provisioning profile (`.mobileprovision` file)

### Builder

`ios-signer-service` offloads the signing process to a dedicated macOS builder. This step is necessary because signing is only officially supported on a macOS system. While third-party cross-platform alternatives exist, they are not as stable or quick to update as the official solution.

However, you don't need to own a Mac! In fact, you don't even need to pay anything. [GitHub Actions](https://docs.github.com/en/actions), [Semaphore CI](https://semaphoreci.com/), and some other CI providers give you free monthly allowance to use a macOS VM and build your projects. In this case, you will be using them to sign your apps.

An implementation for GitHub Actions and Semaphore CI can be found at [ios-signer-ci](https://github.com/SignTools/ios-signer-ci). To host it, simply fork the repo and follow its README.

You can always use another CI provider, or even your own machine, given it supports remote workflow triggers over API. You will see the requirements in the configuration section below.

### Web Service

`ios-signer-service` (this project) is a web service that you install on any server. The service exposes a web interface which allows the user to upload unsigned app files and have them signed using any of the configured signing profiles.

The easiest way to get the service is by downloading the pre-compiled binaries from the [releases](https://github.com/SignTools/ios-signer-service/releases).

You can also use the official [Docker image](https://hub.docker.com/r/signtools/ios-signer-service), but make sure to configure it properly:

- Mount the configuration file to your host, so you can edit it after it's generated. The file's location in the container is just under the root directory: `/signer-cfg.yml`.
- Mount the directory that you will use for the app's data. By default, the location in the container is: `/data`. You can set this path using the `save_dir` property in the configuration file.

The default port used by the service is `8080`. You can override this by running the service with an argument `-port 1234`, where "1234" is your desired port. You can see a description of these arguments and more via `-help`.

`ios-signer-service` is not designed to run by itself - it does not offer encryption (HTTPS) or global authentication. This is a huge security issue, and OTA installations will not work! Instead, you have two options:

- **Reverse proxy** (recommended)

  Run a reverse proxy like nginx, which wraps the service with HTTPS and authentication. You will need a valid HTTPS certificate (self-signed won't work with OTA due to Apple restriction), which means that you will also need a domain. While this setup is more involved, it is the industry-standard way to deploy any web application. It is also the most unrestricted, reliable and secure method by far.

  You must leave a few endpoints non-authenticated, as they are used by OTA and the builder. Don't worry, they are secured by long ids and/or the workflow key:

  ```
  /apps/:id/
  /jobs
  /jobs/:id
  ```

  (where `:id` is a wildcard parameter)

- **ngrok**

  If you are just testing or can't afford the option above, you can also use [ngrok](https://ngrok.com). It offers a free plan that allows you to create a publicly accessible tunnel to your service, conveniently wrapped in ngrok's valid HTTPS certificate. Note that the free plan has a limit of 40 connections per minute, and the URLs change every time you restart ngrok, so you will have to remember to update them.

  To use ngrok, install the program, then just run the following command:

  ```bash
  ngrok http -inspect=false 8080
  ```

  You will get two URLs - make sure to always use the HTTPS one, both when configuring and when using the service.
  Also, make sure to enable `basic_auth` in the configuration below, or anybody could access your service.

When you run the service for the first time, it will exit immediately and generate a configuration file, `signer-cfg.yml`. Take your time to read through it and set it appropriately - the default's won't work!

An explanation of the settings:

```yml
# the builder's signing workflow
workflow:
  # an API endpoint that will trigger a run of the signing workflow
  trigger:
    # your builder's trigger url
    url: https://api.github.com/repos/foo/bar/actions/workflows/sign.yml/dispatches
    # data to send with the trigger request
    body: '{"ref":"master"}'
    # headers to send with the trigger request
    headers:
      # usually you'll add the CI's token here
      Authorization: Token MY_TOKEN
      # either json or form
      Content-Type: application/json
    # whether to attempt http2 or stick to http1
    # set to false if using Semaphore CI
    attempt_http2: true
  # a url that will be open when you click on "Status" in the website while a sign job is running
  status_url: https://github.com/foo/bar/actions/workflows/sign.yml
  # a key that you make up, which will be used by the builder to communicate with the service
  # make sure it is long and secure!
  key: MY_SUPER_LONG_SECRET_KEY
# the public address of your server, used to build URLs for the website and builder
# must be valid HTTPS or OTA won't work!
server_url: https://mywebsite.com
# where to save data like apps and signing profiles
save_dir: data
# apps older than this time will be deleted when a cleanup job is run
cleanup_mins: 10080
# how often does the cleanup job run
cleanup_interval_mins: 30
# protects the web ui with a username and password
# this does not overlap with the "workflow.key" protection
basic_auth:
  enable: false
  username: ""
  password: ""
```

Depending on your builder provider, the `workflow` section will vary. Here are examples of the most popular CI providers:

#### GitHub Actions

```yml
workflow:
  trigger:
    url: https://api.github.com/repos/YOUR_PROFILE/ios-signer-ci/actions/workflows/sign.yml/dispatches
    body: '{"ref":"master"}'
    headers:
      Authorization: Token YOUR_TOKEN
      Content-Type: application/json
    attempt_http2: true
  status_url: https://github.com/YOUR_PROFILE/ios-signer-ci/actions/workflows/sign.yml
```

#### Semaphore CI

```yml
workflow:
  trigger:
    url: https://YOUR_PROFILE.semaphoreci.com/api/v1alpha/plumber-workflows
    body: project_id=YOUR_PROJECT_ID&reference=refs/heads/master
    headers:
      Authorization: Token YOUR_TOKEN
      Content-Type: application/x-www-form-urlencoded
    attempt_http2: false
  status_url: https://YOUR_PROFILE.semaphoreci.com/projects/ios-signer-ci
```

Inside the `save_dir` directory from your configuration ("data" by default), you need to add at least one code signing profile. The structure is as follows:

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

When an app is uploaded to the service for signing, a signing job is generated and stored in memory. The service then triggers the builder using the configured workflow trigger API. The builder will query the available jobs from the service using the `/jobs` endpoint, and download the most recent job's data. This data is a simple TAR file which contains all the necessary signing files. When the builder is finished, it will upload the signed file to the service using a "return id" found within the archive.

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

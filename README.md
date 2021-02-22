# iOS Signer Service

> A cross-platform, self-hosted service to sign iOS apps using GitHub CI

## Introduction

Not all apps are allowed to exist on the App Store. You may want to keep your personal apps to yourself. When sideloading is so severely hindered by Apple, you need a better way to get things done.

Enter `ios-signer-service` - a cross-platform, self-hosted web service to sign iOS apps and effortlessly install them on your device, without a computer.

**NOTICE**: You MUST have a valid **signing certificate** and **provisioning profile**. This project does not help you install apps for free.

## Features

- No jailbreak required
- All iOS versions supported
- No computer required (apart from server to host the service)
- Minimalistic, mobile-friendly web interface
- Upload unsigned apps, download signed apps
- Install signed apps straight to your iOS device via [OTA](https://medium.com/@adrianstanecki/distributing-and-installing-non-market-ipa-application-over-the-air-ota-2e65f5ea4a46)
- Choose from multiple signing profiles
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

- A server; even a Raspberry Pi will work
- Reverse proxy with valid HTTPS (self-signed will NOT work with OTA)
- GitHub account
- Signing certificate (`.p12` file)
- Provisioning profile (`.mobileprovision` file)

### Service

`ios-signer-service` (this project) is a web service that you install on any server. The service exposes a web interface which allows the user to upload "unsigned" app files and have them signed using any of the configured signing profiles.

The easiest way to install the service is using the [Docker image](https://hub.docker.com/r/signtools/ios-signer-service).

When you run the program for the first time, it will exit immediately and generate a configuration file. Make sure you set it appropriately.

Inside the `save_dir` directory ("data" by default), you need to add at least one code signing profile. The structure is as follows:

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
|____apps                      # managed by the service
| |____...
```

By default, `ios-signer-service` does not offer any kind of authentication. This is a security issue - anybody can download and tamper with your apps and even signing certificates! Instead, run a reverse-proxy, like nginx, and wrap the service with authentication. The only endpoints you must leave non-authenticated (used for OTA and CI) are as follows (`:id` is a wildcard parameter):

```
/app/:id/
/profile/:id/
```

When information is passed to the CI, one-time IDs are generated instead of the actual app and profile IDs. When a request to them is processed, the IDs will be destroyed. This mitigates the fact that GitHub's CI will inevitably leak (print) the environment variables passed when the workflow is started, which includes the URL suffixes.

### CI

Uploaded "unsigned" app files are sent from `ios-signer-service` to `ios-signer-ci` for signing. This offloading is necessary because signing is only supported on a macOS system. `ios-signer-ci` uses GitHub's CI to sign the file, since it offers a macOS environment, and then sends it back to `ios-signer-service`. Finally, the user is able to download or install the signed app from the same web interface where they uploaded it.

To host your own [ios-signer-ci](https://github.com/SignTools/ios-signer-ci), simply fork the repo and follow its README.

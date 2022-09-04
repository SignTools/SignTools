# Frequently Asked Questions (F.A.Q.)

## Table of Contents

- [Frequently Asked Questions (F.A.Q.)](#frequently-asked-questions-faq)
  - [Table of Contents](#table-of-contents)
  - [Free developer account limitations](#free-developer-account-limitations)
    - [1. Apps cannot be installed Over the Air (OTA)](#1-apps-cannot-be-installed-over-the-air-ota)
    - [2. You must manually register your device(s) to the developer account](#2-you-must-manually-register-your-devices-to-the-developer-account)
    - [3. Two-factor authentication (2FA)](#3-two-factor-authentication-2fa)
    - [4. Each signed app will expire in 7 days](#4-each-signed-app-will-expire-in-7-days)
    - [5. A maximum of 10 app ids can be registered per 7 days](#5-a-maximum-of-10-app-ids-can-be-registered-per-7-days)
  - [Different types of certificates/provisioning profiles](#different-types-of-certificatesprovisioning-profiles)
    - [1. Certificates](#1-certificates)
    - [2. Provisioning profiles](#2-provisioning-profiles)
  - [Service troubleshooting](#service-troubleshooting)
    - [1. App runs, but malfunctions due to invalid signing/entitlements](#1-app-runs-but-malfunctions-due-to-invalid-signingentitlements)
    - [2. "This app cannot be installed because its integrity could not be verified."](#2-this-app-cannot-be-installed-because-its-integrity-could-not-be-verified)
    - [3. "Unable To Install \*.ipa"](#3-unable-to-install-ipa)
    - [4. Install button does not work](#4-install-button-does-not-work)
  - [Heroku troubleshooting](#heroku-troubleshooting)
    - [1. Changing existing configuration variables](#1-changing-existing-configuration-variables)
    - [2. Retrieving logs via the UI](#2-retrieving-logs-via-the-ui)

## Free developer account limitations

### 1. Apps cannot be installed Over the Air (OTA)

Aka "Install button doesn't work", "Unable to install \*.ipa". This is a deliberate restriction by Apple, not a bug. Open the signer service page on your computer, click "Download", then sideload the app manually.

### 2. You must manually register your device(s) to the developer account

For each device where you want to sideload apps, you need to have installed any app signed with your developer account at least once manually before using this service. Doing so will register your device's identifier (UDID) with the developer account, something the builder cannot do without physical connection with your device.

**On macOS**: You can just build a blank new app or [SimpleApp](https://github.com/SignTools/SignTools-CI/tree/master/SimpleApp) and run it on your phone. That will take care of UDID registration.

**On all other platforms**: You can install any app with a third-party signing tool like [AltStore](https://altstore.io/). That will take care of UDID registration.

### 3. Two-factor authentication (2FA)

Upon submitting an app for signing, you will be redirected to the index page (dashboard), where you will see the new app as "processing". If the account used to sign this app requires a 2FA code, in next minute it will be sent to you by Apple. If this happens, click the `Submit 2FA` button on your app in the dashboard and enter the code you just received. It will be used by the builder to finish logging into your account and perform the signing.

If you use one of the CI builders, each time you sign an app a new computer will be added as "signed in" to your account. Currently, there is no way to automatically sign out a builder after it's done. You can always remove these computers manually, either from your Apple device or [appleid.apple.com](https://appleid.apple.com/). If you are uncomfortable with this, use a separate Apple account.

### 4. Each signed app will expire in 7 days

Resign it and you will get another 7 days.

### 5. A maximum of 10 app ids can be registered per 7 days

Re-use an existing app's bundle id if you hit the limit. Note that the old app will be replaced with the new one when you install it. Otherwise, wait for an app id to expire.

## Different types of certificates/provisioning profiles

### 1. Certificates

- **Apple Development**

  This is the default certificate type. If you have a free developer account, it grants you access to some standard entitlements, including app debugging (`get-task-allow`), which is necessary for some jailbreaks and emulators. If you have a paid developer account, it grants you access to all standard entitlements. This is the recommended type of certificate to use.

- **Apple Distribution**

  This certificate type is only available to paid developer accounts, as it is used when publishing an app. It grants you access to every standard entitlement, except for app debugging (`get-task-allow`). Additionally, it allows you to use production entitlements such as push notifications. Only use this type of certificate if you need those extra entitlements.

### 2. Provisioning profiles

- **Wildcard**

  Its `application-identifier` looks like `TEAM_ID.*`. It can properly sign any app (`TEAM_ID.app1`, `TEAM_ID.app2`, ...), but it can't contain most standard entitlements such as app groups or iCloud containers.

- **Explicit**

  Its `application-identifier` looks like `TEAM_ID.app1`. It can properly sign only one app (`TEAM_ID.app1`), but it can contain any standard entitlement. You can also improperly sign any app with any id, but some functions such as file importing will not work.

## Service troubleshooting

### 1. App runs, but malfunctions due to invalid signing/entitlements

First, make sure you are signing the app correctly and not breaking the entitlements. Read the [types of certificates and profiles](#different-types-of-certificatesprovisioning-profiles) section.

If that doesn't help, you need to figure out what entitlements the app requires. unc0ver 6.0.2 and DolphiniOS emulator need the app debugging (`get-task-allow`) entitlement. Make sure you are using a signing profile with `get-task-allow=true` in its provisioning profile. Also, when you upload such an app to this service, make sure to tick the `Enable app debugging` option. Since this is a potential security issue, it will be disabled by default unless you tick the box.

### 2. "This app cannot be installed because its integrity could not be verified."

This error means that the signature is invalid. Is your signing profile valid? Is your device's UDID registered with the signing profile? To debug this problem, install [libimobiledevice](https://libimobiledevice.org/) (for Windows: [imobiledevice-net](https://github.com/libimobiledevice-win32/imobiledevice-net)). Download the problematic signed app from your service to your computer, then attempt to install it on your iOS device:

```bash
ideviceinstaller -i app.ipa
```

You can also use `-u YOUR_UDID -n` to run this command over the network. When the installation finishes, you should see a more detailed error. Please create an issue here on GitHub and upload the unsigned app along with the detailed error from above so this can be fixed.

### 3. "Unable To Install \*.ipa"

This error means that there was a problem while installing the app. Are you trying to web install (OTA) an app signed with a free developer account? That's sadly not possible. Read the [free account limitations](#free-developer-account-limitations) section.

Otherwise, try installing again, sometimes it's a network problem. If that doesn't help, refer to the [integrity verification error](#2-this-app-cannot-be-installed-because-its-integrity-could-not-be-verified) section.

### 4. Install button does not work

Check your logs for something among these lines:

> WRN using OTA manifest proxy, installation may not work

If you see the warning, then you are trying to access the service over HTTP instead of HTTPS. Apple only allows OTA installation over HTTPS, so to make it work for you, a special manifest proxy is used. The server that delivers the proxy is limited to 100,000 requests per day globally, so unfortunately the limit has likely been reached. Wait one day, or access your service over HTTPS instead.

## Heroku troubleshooting

### 1. [Changing existing configuration variables](https://devcenter.heroku.com/articles/config-vars#using-the-heroku-dashboard)

### 2. [Retrieving logs via the UI](https://devcenter.heroku.com/articles/logging#log-retrieval-via-the-ui)

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
    - [6. You cannot use an existing app's bundle id](#6-you-cannot-use-an-existing-apps-bundle-id)
  - [What kind of certificates/provisioning profiles are supported?](#what-kind-of-certificatesprovisioning-profiles-are-supported)
  - [App runs, but malfunctions due to invalid signing/entitlements](#app-runs-but-malfunctions-due-to-invalid-signingentitlements)
  - ["This app cannot be installed because its integrity could not be verified."](#this-app-cannot-be-installed-because-its-integrity-could-not-be-verified)
  - ["Unable To Install \*.ipa"](#unable-to-install-ipa)
  - [How can I debug a failing builder?](#how-can-i-debug-a-failing-builder)
  - [Install button does not work](#install-button-does-not-work)

## Free developer account limitations

### 1. Apps cannot be installed Over the Air (OTA)

Aka "Install button doesn't work", "Unable to install \*.ipa". This is a deliberate restriction by Apple, not a bug. Open the signer service page on your computer, click "Download", then sideload the app manually.

### 2. You must manually register your device(s) to the developer account

For each device where you want to sideload apps, you need to have installed any app signed with your developer account at least once manually before using this service. Doing so will register your device's identifier (UDID) with the developer account, something the builder cannot do without physical connection with your device.

**On macOS**: You can just build a blank new app or [SimpleApp](https://github.com/SignTools/ios-signer-ci/tree/master/SimpleApp) and run it on your phone. That will take care of UDID registration.

**On all other platforms**: You can install any app with a third-party signing tool like [AltStore](https://altstore.io/). That will take care of UDID registration.

### 3. Two-factor authentication (2FA)

Upon submitting an app for signing, you will be redirected to the index page (dashboard), where you will see the new app as "processing". If the account used to sign this app requires a 2FA code, in next minute it will be sent to you by Apple. If this happens, click the `Submit 2FA` button on your app in the dashboard and enter the code you just received. It will be used by the builder to finish logging into your account and perform the signing.

If you use one of the CI builders, each time you sign an app a new computer will be added as "signed in" to your account. Currently, there is no way to automatically sign out a builder after it's done. You can always remove these computers manually, either from your Apple device or [appleid.apple.com](https://appleid.apple.com/). If you are uncomfortable with this, use a separate Apple account.

### 4. Each signed app will expire in 7 days

Re-sign it and you will get another 7 days.

### 5. A maximum of 10 app ids can be registered per 7 days

Re-use an existing app's bundle id if you hit the limit. Note that the old app will be replaced with the new one when you install it. Otherwise, wait for an app id to expire.

### 6. You cannot use an existing app's bundle id

Say you were feeling adventurous and wanted to sign an app with the same bundle id as YouTube. You can't do that with a free developer account. Apple checks if the bundle id is already registered anywhere else before providing you with a provisioning profile for that id.

## What kind of certificates/provisioning profiles are supported?

Technically, everything is supported as long as your iOS device trusts it. This includes free signing profiles, but of course, they expire after a week. The only major difference between signing profiles is based on the provisioning profile's `application-identifier`. There are two types:

- Wildcard, with app id = `TEAM_ID.*`

  - Can properly sign any app (`TEAM_ID.app1`, `TEAM_ID.app2`, ...)
  - Can't use special entitlements such as app groups or iCloud containers (Apple restriction)

- Explicit, with app id = `TEAM_ID.app1`
  - Can properly sign only one app (`TEAM_ID.app1`)
  - Can use any entitlement as long as it's in the provisioning profile
  - If you sign multiple apps with the same profile, only one of the apps can be installed on your device at a time. This is because their bundle ids will be identical and the apps will replace each other.

## App runs, but malfunctions due to invalid signing/entitlements

First, make sure you are signing the app correctly and not breaking the entitlements. Read the [kinds of certificates and profiles](#what-kind-of-certificatesprovisioning-profiles-are-supported) section.

If that doesn't help, you need to figure out what entitlements the app requires. unc0ver 6.0.2 and DolphiniOS emulator need the app debugging (`get-task-allow`) entitlement. Make sure you are using a signing profile with `get-task-allow=true` in its provisioning profile. Also, when you upload such an app to this service, make sure to tick the `Enable app debugging` option. Since this is a potential security issue, it will be disabled by default unless you tick the box.

## "This app cannot be installed because its integrity could not be verified."

This error means that the signature is invalid. Is your signing profile valid? Is your device's UDID registered with the signing profile? To debug this problem, install [libimobiledevice](https://libimobiledevice.org/) (for Windows: [imobiledevice-net](https://github.com/libimobiledevice-win32/imobiledevice-net)). Download the problematic signed app from your service to your computer, then attempt to install it on your iOS device:

```bash
ideviceinstaller -i app.ipa
```

You can also use `-u YOUR_UDID -n` to run this command over the network. When the installation finishes, you should see a more detailed error. Please create an issue here on GitHub and upload the unsigned app along with the detailed error from above so this can be fixed.

## "Unable To Install \*.ipa"

This error means that there was a problem while installing the app. Are you trying to web install (OTA) an app signed with a free developer account? That's sadly not possible. Read the [free account limitations](#free-developer-account-limitations) section.

Otherwise, try installing again, sometimes it's a network problem. If that doesn't help, refer to the [integrity verification error](#this-app-cannot-be-installed-because-its-integrity-could-not-be-verified) section.

## How can I debug a failing builder?

Check the builder's logs for any errors. You can get to the logs by clicking the "Status" button on any app in the web interface while it's signing or failed.

## Install button does not work

Check your logs for something among these lines:

> WRN using OTA manifest proxy, installation may not work

If you see the warning, then you are trying to access the service over HTTP instead of HTTPS. Apple only allows OTA installation over HTTPS, so to make it work for you, a special manifest proxy is used. The server that delivers the proxy is limited to 100,000 requests per day globally, so unfortunately the limit has likely been reached. Wait one day, or access your service over HTTPS instead.

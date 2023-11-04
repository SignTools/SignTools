# Frequently Asked Questions (F.A.Q.)

## Table of Contents

- [Frequently Asked Questions (F.A.Q.)](#frequently-asked-questions-faq)
  - [Table of Contents](#table-of-contents)
  - [Different types of certificates/provisioning profiles](#different-types-of-certificatesprovisioning-profiles)
    - [1. Certificates](#1-certificates)
    - [2. Provisioning profiles](#2-provisioning-profiles)
  - [Service troubleshooting](#service-troubleshooting)
    - [1. App runs, but malfunctions due to invalid signing/entitlements](#1-app-runs-but-malfunctions-due-to-invalid-signingentitlements)
    - [2. "This app cannot be installed because its integrity could not be verified."](#2-this-app-cannot-be-installed-because-its-integrity-could-not-be-verified)
    - [3. "Unable To Install \*.ipa"](#3-unable-to-install-ipa)
    - [4. Install button does not work](#4-install-button-does-not-work)

## Different types of certificates/provisioning profiles

### 1. Certificates

- **Apple Development**

  This is the default certificate type. It grants you access to all standard entitlements, including app debugging (`get-task-allow`), which is necessary for some jailbreaks and emulators. This is the recommended type of certificate to use.

- **Apple Distribution**

  This certificate type is used when publishing an app. It grants you access to every standard entitlement, except for app debugging (`get-task-allow`). Additionally, it allows you to use production entitlements such as push notifications. Only use this type of certificate if you need those extra entitlements.

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

Try installing again, sometimes it's a network problem. If that doesn't help, refer to the [integrity verification error](#2-this-app-cannot-be-installed-because-its-integrity-could-not-be-verified) section.

### 4. Install button does not work

Check your logs for something among these lines:

> WRN using OTA manifest proxy, installation may not work

If you see the warning, then you are trying to access the service over HTTP instead of HTTPS. Apple only allows OTA installation over HTTPS, so to make it work for you, a special manifest proxy is used. The server that delivers the proxy is limited to 100,000 requests per day globally, so unfortunately the limit has likely been reached. Wait one day, or access your service over HTTPS instead.

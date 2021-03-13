# Frequently Asked Questions (F.A.Q.)

## Free developer account limitations

### 1. Apps cannot be installed Over the Air (OTA)

Aka "Install button doesn't work", "Unable to install \*.ipa". This is a deliberate restriction by Apple, not a bug. Open the signer service page on your computer, click "Download", then sideload the app manually.

### 2. You must manually register your device(s) to the developer account

For each device where you want to sideload apps, you need to have installed any app signed with your developer account at least once manually before using this service. Doing so will register your device's identifier (UDID) with the developer account, something the builder cannot do without physical connection with your device.

First, follow the `How do I get the certificate?` section below.

**On macOS**: You can just build a blank new app or [SimpleApp](https://github.com/SignTools/ios-signer-ci/tree/master/SimpleApp) and run it on your phone. That will take care of UDID registration.

**On all other platforms**: You can install any app with a third-party signing tool like [AltStore](https://altstore.io/). That will take care of UDID registration.

### 3. Two-factor authentication (2FA)

Upon submitting an app for signing, the web service will ask you for a 2FA code. It will be used by the builder to log into your account and perform the signing.

If you use one of the CI builders, each time you sign an app a new computer will be added as "signed in" to your account. Currently, there is no way to automatically sign out a builder after it's done. You can always remove these computers manually, either from your Apple device or [appleid.apple.com](https://appleid.apple.com/). If you are uncomfortable with this, use a separate Apple account.

### 4. Each signed app will expire in 7 days

Re-sign it and you will get another 7 days.

### 5. A maximum of 10 app ids can be registered per 7 days

Re-use an existing app's bundle id if you hit the limit. Note that the old app will be replaced with the new one when you install it. Otherwise, wait for an app id to expire.

### 6. You cannot use an existing app's bundle id

Say you were feeling adventurous and wanted to sign an app with the same bundle id as YouTube. You can't do that with a free developer account. Apple checks if the bundle id is already registered anywhere else before providing you with a provisioning profile for that id.

## How do I get a certificate?

**On macOS:** Install [Xcode](https://developer.apple.com/xcode/) and open the `Account Preferences` (A). Sign into your account using the plus button. Select your account and click on `Manage Certificates...`. In the new window (B), click the plus button and then `Apple Development`. Click `Done`. Now open the `Keychain` app (C). There you will find your certificate and private key. Select them by holding `Command`, then right-click and select `Export 2 items...`. This will export you the `.p12` file you need.

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

**On all other platforms:** There is no official way to do this. You should be able to use a third-party signing tool like [AltStore](https://altstore.io/) and then extract the certificate from its app data, but this has not been tested.

## What kind of certificates/provisioning profiles are supported?

Technically, everything is supported as long as your iOS device trusts it. This includes free signing profiles, but of course, they expire after a week. The only major difference between signing profiles is based on the provisioning profile's `application-identifier`. There are two types:

- Wildcard, with app id = `TEAM_ID.*`

  - Can properly sign any app (`TEAM_ID.app1`, `TEAM_ID.app2`, ...)
  - Can't use special entitlements such as app groups (Apple restriction)

- Explicit, with app id = `TEAM_ID.app1`
  - Can properly sign only one app (`TEAM_ID.app1`)
  - Can use any entitlement as long as it's in the provisioning profile
  - If you properly sign multiple apps with the same profile, only one of the apps can be installed on your device at a time. This is because their bundle ids will be identical and the apps will replace each other.
  - It appears possible, at least on iOS 14.4, to improperly sign apps by mismatching their bundle id and profile app id, and the apps will still run. For an example, with an app id `TEAM_ID.app1`, you could sign the apps `TEAM_ID.app2` and `TEAM_ID.app3`. This way you can have multiple apps installed at the same time, but all of their entitlements will be broken, including file importing. Does not work with free developer accounts.

## App runs, but malfunctions due to invalid signing/entitlements

First, make sure you are signing the app correctly and not breaking the entitlements. Read the section just above.

If that doesn't help, you need to figure out what entitlements the app requires. unc0ver 6.0.2 and DolphiniOS emulator need the app debugging (`get-task-allow`) entitlement. Make sure you are using a signing profile with `get-task-allow=true` in its provisioning profile. Also, when you upload such an app to this service, make sure to tick the `Enable app debugging` option. Since this is a potential security issue, it will be disabled by default unless you tick the box.

## "This app cannot be installed because its integrity could not be verified."

This error means that the signing process went terribly wrong. To debug the problem, install [libimobiledevice](https://libimobiledevice.org/) (for Windows: [imobiledevice-net](https://github.com/libimobiledevice-win32/imobiledevice-net)). Download the problematic signed app from your service to your computer, and then attempt to install it on your iOS device:

```bash
ideviceinstaller -i app.ipa
```

You can also use `-u YOUR_UDID -n` to run this command over the network. When the installation finishes, you should see a more detailed error. Please create an issue here on GitHub and upload the unsigned app along with the detailed error from above so this can be fixed.

## "Unable To Install \*.ipa"

Are you trying to OTA install an app signed with a free developer account? That's sadly not possible. Read the `Free developer account limitations` section above.

Otherwise, try installing again, sometimes it's a network problem. If that doesn't help, refer to the `This app cannot be installed because its integrity could not be verified` section above.

## How can I debug a failing builder?

Edit the `sign.sh` file in your builder's repo and remove the output suppression from the failing line. Usually this will be the `xresign.sh` call, so:

```bash
./xresign.sh ...  >/dev/null 2>&1
```

Becomes:

```bash
./xresign.sh ...
```

Next time you run a build, the logs will give you full details that you can use to resolve your issue. The reason that the output suppression is there in the first place is to prevent leaks of potentially sensitive information about your certificates and apps.

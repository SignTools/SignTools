# Simple Installation

For a video tutorial, [click here](https://youtu.be/mOmEcaFtBgk). Note that you still need this written guide - use the video as an addition, but not substitution for the instructions below.

## 1. Limitations

This installation guide uses [Heroku](https://www.heroku.com/) to host your signing service for free. The free plan has several small limitations that you should be aware of:

- You can only run your service for 550 hours per month (23 days).
- HOWEVER, your service will automatically turn off after 30 minutes of inactivity. When off, usage is not counted. If you access the service when it is off, it will automatically turn on again.
- Storage is deleted every time the service is turned off. This effectively means that signed apps are kept for 30 minutes.

For more information, check the [pricing page](https://www.heroku.com/pricing).

## 2. Builder

You will have to create a builder. Currently, only GitHub Actions is supported. Head over to [ios-signer-ci](https://github.com/SignTools/ios-signer-ci) and follow the GitHub Actions instructions.

Once you have made your builder, proceed below.

## 3. Signing profile

You need a signing profile to be able to sign apps. A signing profile is simply a collection of files and credentials that Apple provides to developers so they can sign apps.

There are two types of signing profiles:

- **Developer account** (recommended)

  This method works for both free and paid developer accounts. You only need your Apple account's name and password. You will likely be prompted for a 6-digit code every time you sign an app, which you can submit on the service's web page. This method will be able to use most entitlements, resulting in working app extensions and iCloud synchronization. There are no restrictions if you have a paid account. If you have a free account, make sure you read and understand the limitations in the [FAQ](FAQ.md#free-developer-account-limitations) page.

- **Manual provisioning profile**

  If you don't have a paid developer account, but you have a manual provisioning profile with a `.mobileprovision` extension, you can use this method instead. Based on the type of your provisioning profile, different entitlements and features may not work on your signed apps. For the differences, check the [FAQ](FAQ.md#what-kind-of-certificatesprovisioning-profiles-are-supported) page.

Additionally, you will also need a certificate file with a `.p12` extension. If you are using a manual provisioning profile, you likely received a certificate along with it - use that. Otherwise, follow the instructions below:

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

## 4. Web service

Click on the button below - register for a free account and follow the setup page.

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/SignTools/ios-signer-service/tree/master)

Once you are done, you will be left with your very own web service. Congratulations!

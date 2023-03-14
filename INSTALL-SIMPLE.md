# Simple Installation

## Video tutorial

For a video tutorial, [click here](https://youtu.be/mOmEcaFtBgk). **You still need this written guide** — the video is not up to date, and it does not cover everything written here.

## 1. Limitations

This installation guide uses [fly.io](https://fly.io/) to host your signing service for free. The free plan has a couple of minor limitations to be aware of:

 - Up to 3 shared-cpu-1x 256mb VMs
 - 3GB persistent volume storage (total)
 - 160GB outbound data transfer
 - You may need to provide them with a credit card number in order to sign up
 
More information is available on their [pricing page](https://fly.io/docs/about/pricing/#free-allowances)

## 2. Builder

You will have to create a builder. Currently, only GitHub Actions is supported. Head over to [SignTools-CI](https://github.com/SignTools/SignTools-CI) and follow the **GitHub Actions** instructions.

Once you have made your builder, proceed below.

## 3. Signing profile

You need a signing profile to be able to sign apps. A signing profile is simply a collection of files and credentials that Apple provides to developers so they can sign apps.

There are two types of signing profiles:

- **Developer account**

  This method works for both free and paid developer accounts. You only need your Apple account's name and password. You will likely be prompted for a 6-digit code every time you sign an app, which you can submit on the service's web page. This method will be able to use most entitlements, resulting in working app extensions and iCloud synchronization. There are no restrictions if you have a paid account. If you have a free account, make sure you read and understand the limitations in the [FAQ](FAQ.md#free-developer-account-limitations) page.

- **Custom provisioning profile**

  If you have a provisioning profile with a `.mobileprovision` extension, you can use this method as well. There is no 6-digit code, so signing will be faster than a developer account. However, based on the type of your provisioning profile, different entitlements and features may not work on your signed apps. For the differences, check the [FAQ](FAQ.md#what-kind-of-certificatesprovisioning-profiles-are-supported) page.

Additionally, you will also need a certificate archive with a `.p12` extension. It must contain at least one certificate and at least one private key. If you need development entitlements, add an `Apple Development` certificate and its key. If you need distribution entitlements, add both an `Apple Development` and `Apple Distribution` certificate, along with their keys. For the differences, check the [FAQ](FAQ.md#what-kind-of-certificatesprovisioning-profiles-are-supported) page.

If you are using a custom provisioning profile, you likely received a certificate archive along with it — use that. If you have a developer account, you can create one from the [developer portal](https://developer.apple.com/account/resources/certificates/list). Otherwise, follow the instructions below:

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

In order to get your web service online, follow these steps to get set up:

 - [Install the Fly.io CLI](https://fly.io/docs/hands-on/install-flyctl/)
 - [Sign up for Fly.io](https://fly.io/docs/hands-on/sign-up/)
 - [Login to your account with the CLI](https://fly.io/docs/hands-on/sign-in/)
 - Run `git clone https://github.com/SignTools/SignTools.git` on your computer that has the Fly CLI 
 - `cd SignTools`
 - Run the helper script to assist with generating your `fly.toml` file: `python3 fly-config-builder.py`
 - After the script has completed successfully, run `fly launch`, and once it's done building, you should be good to go!
 - (optional) Head to [fly.io/dashboard](https://fly.io/dashboard) now if you'd like to setup a custom domain with an SSL certificate
 
## 5. Troubleshooting

Check out the [FAQ](FAQ.md) page.

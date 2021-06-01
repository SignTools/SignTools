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

You will need to prepare a signing profile for use with the signing service.
There are two types of signing profiles:

- **Certificate + provisioning profile**

  If you have a paid developer account, it is highly recommended to use this method. Doing so will save you from a lot of limitations. To get a provisioning profile (`.mobileprovision` file), [create one](https://developer.apple.com/library/archive/recipes/ProvisioningPortal_Recipes/CreatingaDevelopmentProvisioningProfile/CreatingaDevelopmentProvisioningProfile.html) from your developer portal and download it. You will probably want it to be a `Development` type and not `Distribution`, so that you can have a `wildcard` application identifier and app debugging entitlement (`get-task-allow`). For the differences, check the [FAQ](FAQ.md) page. Also don't forget to [register the UDID](https://developer.apple.com/library/archive/recipes/ProvisioningPortal_Recipes/AddingaDeviceIDtoYourDevelopmentTeam/AddingaDeviceIDtoYourDevelopmentTeam.html#//apple_ref/doc/uid/TP40011211-CH1-SW1) of each device that you want to sideload to. Read ahead on how to get your certificate.

- **Certificate + developer account**

  If you don't have a paid developer account, this is your only option. Make sure to read and understand the limitations in the [FAQ](FAQ.md) page before you proceed. Read ahead on how to get your certificate.

The certificate is a file with an extension `.p12`. To obtain it, follow the instructions below:

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

**On Windows:** There is no official way to do this. However, you can use [altserver-cert-dumper](https://github.com/SignTools/altserver-cert-dumper) as a workaround.

## 4. Web service

Click on the button below - register for a free account and follow the setup page.

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/SignTools/ios-signer-service/tree/master)

Once you are done, you will be left with your very own web service. Congratulations!

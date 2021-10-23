<p align="center">
    <img width="256" heigth="256" src="img/logo.png">
    <h1 align="center">SignTools</h1>
    <p align="center">
        A self-hosted, cross-platform service to sign and install iOS apps, all <b>without a computer</b>.
    </p>
    <p align="center">
        <img alt="GitHub" src="https://img.shields.io/github/license/signtools/SignTools">
        <img alt="GitHub issues" src="https://img.shields.io/github/issues/signtools/SignTools">
        <img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/signtools/signtools">
        <img alt="Docker Image Version (latest semver)" src="https://img.shields.io/docker/v/signtools/signtools">
        <img alt="GitHub all releases" src="https://img.shields.io/github/downloads/signtools/SignTools/total">
        <img alt="GitHub release (latest SemVer)" src="https://img.shields.io/github/v/release/signtools/SignTools">
    </p>
</p>

## Introduction

There are countless reasons to install apps outside Apple's App Store. Unfortunately, this process is severely hindered and out of reach for most people. Here SignTools comes to the rescue. No more terminal, half-working scripts, or even computer. Sideload any app directly from your phone through a simple web interface.

What's the catch? The workflow uses a separate macOS machine, called a "builder", to perform the signing. This project is only a web service which controls the builder, along with a web interface for you to upload unsigned apps and download signed apps. However, **you don't need to own a Mac** - the builder can be any **free** CI (Continuous Integration) provider or even your own machine. The web service can also be hosted on any **free** server or one of your own.

More information and examples can be found in the installation section.

## :raised_hands: Community

Come join the official Discord to get more interactive support and have general conversations about this project: https://discord.gg/A4T6npnRCk

## Disclaimer

This project is self-hosted; there is no public service. It does not provide any alternative catalog of apps. This project does not provide, promote, or support any form of piracy. This project is aimed solely at people who want to install homebrew apps on their device, much like the popular [AltStore](https://github.com/rileytestut/AltStore). We (all collaborators) cannot be held liable for anything that happens by you using this project.

## Features

- No jailbreak required
- iOS, iPadOS, macOS (M1) supported
- No computer required after an initial setup \*
- Minimalistic, mobile-friendly web interface
- Upload unsigned apps, download signed apps
- Install signed apps from the website straight to your iOS device via [OTA](https://medium.com/@adrianstanecki/distributing-and-installing-non-market-ipa-application-over-the-air-ota-2e65f5ea4a46) \*
- Provisioning profiles, paid and free developer accounts supported
- Every possible signing method supported, including entitlements \*
- Choose from multiple signing profiles for each app

\* Free developer accounts require a computer to install apps and some entitlements will not work. Check out the [FAQ](FAQ.md).

## Screenshots

<table>
<tr>
    <td>
        <img height="600px" src="img/3.png"/>
        <img height="600px" src="img/4.png"/>
    </td>
</tr>
</table>

## Installation

There are multiple ways to install this web service:

- ### [Simple](INSTALL-SIMPLE.md) - on a free Heroku server

- ### [Advanced](INSTALL-ADVANCED.md) - on your own machine

## [Frequently Asked Questions (FAQ)](FAQ.md)

## License

This project and all of its unlicensed dependencies under the [SignTools](https://github.com/SignTools) organization are licensed under AGPL-3.0. A copy of the license can be found [here](LICENSE). Raise an issue if you are interested in exclusive licensing.

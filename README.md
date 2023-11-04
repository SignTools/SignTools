<p align="center">
    <img width="256" heigth="256" src="img/logo.png">
    <h1 align="center">SignTools</h1>
    <p align="center">
        A free, self-hosted platform to sign and install iOS apps without a computer
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

SignTools is a sideloading platform that takes a different approach from any similar tools. It consists of two components — a **service** and a **builder**. The builder is a macOS machine which performs signing using official Apple software. Doing so means high reliability and compatibility. The service (this repo) can be hosted anywhere, and it provides a web interface for you to upload, sign, and download apps, using the builder where necessary. Having the web service means that you don't need anything installed on your phone, and you can still sideload without a computer.

## Note on free developer accounts

Starting from SignTools 3.0.0, free developer accounts are no longer supported. Most of SignTools' unique features have always required a paid account, and using the project with a free account has always been unnecessarily more difficult than other freely available tools. Additionally, certain suboptimal design choices were made to accommodate for free accounts, which reduced stability and performance. Therefore, with this new release, the signing flow has been significantly reworked and should be much faster and more robust.

## :raised_hands: Community

Come join the official Discord to get more interactive support and have general conversations about this project: https://discord.gg/A4T6npnRCk

## Disclaimer

This project is self-hosted; there is no public service. It does not provide any alternative catalog of apps. This project does not provide, promote, or support any form of piracy. This project is aimed solely at people who want to install homebrew apps on their device, much like the popular [AltStore](https://github.com/rileytestut/AltStore). We (all collaborators) cannot be held liable for anything that happens by you using this project.

## Features

- No jailbreak required
- iOS, iPadOS, macOS (native + IPA) supported
- No computer required after an initial setup
- Minimalistic, mobile-friendly web interface
- Upload unsigned apps, download signed apps
- Inject tweaks as you are signing apps
- Install signed apps from the website straight to your iOS device via [OTA](https://medium.com/@adrianstanecki/distributing-and-installing-non-market-ipa-application-over-the-air-ota-2e65f5ea4a46)
- Provisioning profiles and developer accounts supported
- Configurable signing including entitlements
- Choose from multiple signing profiles for each app

## Screenshots

<table>
<tr>
    <td>
        <img height="600px" src="img/3.png"/>
        <img height="600px" src="img/4.png"/>
    </td>
</tr>
</table>

## [Installation](INSTALL.md)

## [Frequently Asked Questions (FAQ)](FAQ.md)

## License

This project and all of its unlicensed dependencies under the [SignTools](https://github.com/SignTools) organization are licensed under AGPL-3.0. A copy of the license can be found [here](LICENSE). Raise an issue if you are interested in exclusive licensing.

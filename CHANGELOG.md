# Changelog

## [3.1.0](https://github.com/SignTools/SignTools/compare/v3.0.13...v3.1.0) (2026-11-11)


### Features

* add reusable tweak library ([f13b34a](https://github.com/SignTools/SignTools/commit/f13b34a53b8d319d9014205a4668ca8c6a69aa9d))
* build the Docker image from scratch instead of Alpine ([7579780](https://github.com/SignTools/SignTools/commit/7579780af7dc3cff3683a55103d66703776fc9d4))
* dark theme with system-following toggle ([62dfb9b](https://github.com/SignTools/SignTools/commit/62dfb9bc148b443ed49776ef13526536e72bab94))
* extract app icons from asset catalogs ([7f5cd48](https://github.com/SignTools/SignTools/commit/7f5cd4879376e4b75be4969f1199a0958f287afc))
* hide the install button for Mac apps ([7dba7ee](https://github.com/SignTools/SignTools/commit/7dba7ee41f91e301befd4907ff2b5611ac98909d))
* hide the redundant signed status on app cards ([64ff2b9](https://github.com/SignTools/SignTools/commit/64ff2b91889443184609c2a8b407a3442894d07f))
* link every status button through the app's status page ([a9044c1](https://github.com/SignTools/SignTools/commit/a9044c114874c6ba6ab2d35451cee2d7bd15bfb5))
* publish a dev Docker image for every push to master ([0684239](https://github.com/SignTools/SignTools/commit/068423957990081333543de28fdc6dd775aeae55))
* put card actions on the same row as the date ([c4fad77](https://github.com/SignTools/SignTools/commit/c4fad77b6afd4889917465cbe0bd3bf08533dd81))
* realtime UI updates via server-sent events ([8ec7648](https://github.com/SignTools/SignTools/commit/8ec76486d62142581d8b4bcf3faa12fa7f08b1b6))
* serve favicon with etag and gzip static assets ([d6fc8ad](https://github.com/SignTools/SignTools/commit/d6fc8ad7a0e05c04d828bf642f55dadfdbfeacad))
* serve web libraries from the binary instead of CDNs ([dc460f5](https://github.com/SignTools/SignTools/commit/dc460f5068e856fc84fac950389e365542b23ded))
* show app icons on cards ([349b013](https://github.com/SignTools/SignTools/commit/349b013c22ff6cdf4f07bf8e53e95b14d5ad7341))
* show self-hosted builder status and live job logs without exposing the builder key ([6729055](https://github.com/SignTools/SignTools/commit/6729055eb44d7f11f328c850b38c16baccd7cb36))
* show the app version on cards ([8cb235f](https://github.com/SignTools/SignTools/commit/8cb235f5361969cdf290f062159bbbbe91a6b153))
* show tweak file names when hovering or tapping an app's tweak count ([448358f](https://github.com/SignTools/SignTools/commit/448358fa39ee9f250bcbaf327af6a2c30753c520))
* toggle between one-off and library tweaks when signing ([855eb37](https://github.com/SignTools/SignTools/commit/855eb3746565d86cb15ffc2613be9dcf5cf3ffcb))
* use icons for the Apps/Tweaks slider, upload and clear search buttons ([18ba0ba](https://github.com/SignTools/SignTools/commit/18ba0ba6015979be02c66533d37347235713588d))


### Bug Fixes

* bump deps ([f2d82d5](https://github.com/SignTools/SignTools/commit/f2d82d5fde1aca6f8883883c3387eff870de3597))
* decode grayscale, gzip and LZVN asset catalog icons ([845bbd0](https://github.com/SignTools/SignTools/commit/845bbd02438ba51f680086ab5bb6d88b3be3b662))
* don't change an app's date when reading its icon and version later ([dbc2c0e](https://github.com/SignTools/SignTools/commit/dbc2c0e4ebb651803fa0d941d075e3cc9c2fcc72))
* find icons that Info.plist lists in subfolders ([d6f758b](https://github.com/SignTools/SignTools/commit/d6f758bf3b46ab3aa25d0f283f19b82d45907ca7))
* keep cards in the same order when apps or tweaks tie ([1cfcd00](https://github.com/SignTools/SignTools/commit/1cfcd00206052f2fe87ccde44d3453479c655c92))
* match the gap below app icons to the title's ([9ea4582](https://github.com/SignTools/SignTools/commit/9ea4582c3d135a68abf45302d9ca39740aa5ab23))
* nil pointer panic when file_url returns non-2xx status ([91c186a](https://github.com/SignTools/SignTools/commit/91c186a8ac86a93309b9da1b0b39e0171160159b))
* pin deps ([f5b0aaf](https://github.com/SignTools/SignTools/commit/f5b0aafa5d23c23993cf5be399ee9b7306d1e9d7))
* re-lay out cards when they change size so they don't overlap ([2604828](https://github.com/SignTools/SignTools/commit/2604828e9fd7eee277092658cc5007e4e7bdbe8c))
* read icons from Mac apps, interlaced PNGs and older asset catalogs ([aa1e65f](https://github.com/SignTools/SignTools/commit/aa1e65f18e925415e7f66f211e9d85c33e0929b3))
* stop animating cards when they move ([ff20997](https://github.com/SignTools/SignTools/commit/ff209971dd715202c0637c0f03415672474a55d1))
* wrap long app names on the install page ([b2b2a12](https://github.com/SignTools/SignTools/commit/b2b2a123dd687fa5698e34974c8e99087c9f6fa4))

## [3.0.13](https://github.com/SignTools/SignTools/compare/v3.0.12...v3.0.13) (2026-11-05)


### Bug Fixes

* bump deps ([6448fb8](https://github.com/SignTools/SignTools/commit/6448fb8ce75b120e37923056d1a112dc0ea161fa))

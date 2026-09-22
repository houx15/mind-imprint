# Personal showcase assets

Four original development illustrations were generated with the built-in image_gen tool (not a direct provider script), then losslessly sourced and encoded to WebP for delivery. Student image generation is a separate application feature routed through the Go gateway draw class and owned OSS storage.

## Generation prompts

Shared constraints: original landscape 3:2 illustration for a teenager personal portfolio header, no text, logos, watermark, interface, or recognizable existing characters.

- clouds: dreamy fluffy cloud garden with small friendly round creatures, soft playful shapes, tiny stars and rounded flowers; gouache and colored-pencil texture, cream pastel pink and mint, storybook kawaii, generous breathing room, pale cream backdrop.
- moon: ivory crescent moon above quiet black-blue mountains, silver stars and orbital paths, subtle purple aurora, fine etched graphic fantasy cover art, cinematic moonlight and rich ink-blue shadows.
- sky: original anime background with luminous blue sky and towering cumulus clouds, small floating island, grass, wind-swept flowers and observatory, hand-painted animation technique, turquoise and warm coral details.
- robot: original friendly exploration robot on a mechanical workbench, articulated joints, metal panels, orange indicators, brushed graphite and ivory metal, isometric industrial concept art, exploded-view parts, charcoal navy backdrop and cyan circuitry, no weapons.

Delivered assets: `apps/lite-web/public/images/showcase/{clouds,moon,sky,robot}.webp`.

## Fonts

Self-hosted full Chinese fonts converted from official Google Fonts TTF sources to WOFF2 without glyph subsetting. SIL Open Font License texts accompany each font in `apps/lite-web/public/fonts/showcase/`.

- [ZCOOL KuaiLe](https://github.com/google/fonts/tree/main/ofl/zcoolkuaile)
- [Ma Shan Zheng](https://github.com/google/fonts/tree/main/ofl/mashanzheng)
- [ZCOOL QingKe HuangYou](https://github.com/google/fonts/tree/main/ofl/zcoolqingkehuangyou)

System sans/serif/mono alternatives remain available. Fonts use swap and are loaded only when used. Minimal style does not require an illustration.

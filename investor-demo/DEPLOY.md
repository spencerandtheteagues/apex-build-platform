# Investor Demo Hosting

This folder is set up to deploy as a free Render static site.

## Fastest path

1. Push the repo to GitHub.
2. In Render, click `New` -> `Blueprint`.
3. Choose the repo.
4. In the Blueprint file path field, enter:

```text
investor-demo/render.static.yaml
```

5. Deploy.

Render will publish the investor demo as a static site and give you a public `onrender.com` URL that opens in one click from a text message.

## What this blueprint does

- Uses `investor-demo` as the Render root directory
- Builds a clean `dist/` folder with only the public demo assets
- Publishes that `dist/` folder as a static site
- Avoids touching the main app's existing `render.yaml`

## Local build

From the repo root:

```bash
cd investor-demo
sh ./build-static-site.sh
```

Then open:

```text
investor-demo/dist/index.html
```

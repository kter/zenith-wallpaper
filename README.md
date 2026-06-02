# zenith-wallpaper

Real-time star-sky desktop wallpaper for sway (Wayland).

Renders what the sky above your location actually looks like at the current moment:
- Milky Way panorama warped to the zenith-centered dome (Lambert azimuthal equal-area)
- Stars from the Yale Bright Star Catalogue (mag ≤ 6.5)
- Planets and Moon at their current positions
- Horizon ring with cardinal directions (N/E/S/W)

Updated hourly via systemd user timer.

## Build & install

```sh
make              # build binary
make install      # copy to /usr/local/bin (requires sudo)
make install-units  # install & enable systemd user timer
```

## Manual run

```sh
zenith-wallpaper
```

## Uninstall

```sh
make uninstall
```

## Data credits

- **Milky Way panorama**: ESO/S. Brunier — [eso0932a](https://www.eso.org/public/images/eso0932a/)  
  © ESO, licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)

- **Yale Bright Star Catalogue (BSC5)**: Hoffleit & Warren (1991).  
  Retrieved from [CDS Strasbourg](https://cdsarc.cds.unistra.fr/ftp/V/50/).  
  Public domain.

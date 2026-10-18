# FFmpeg notice

This container image contains FFmpeg (`ffmpeg`, `ffprobe` and their libraries)
in the **jellyfin-ffmpeg** build, installed from the Jellyfin project's Debian
repository (<https://repo.jellyfin.org/debian>) into `/usr/lib/jellyfin-ffmpeg/`.
The exact package and version are recorded in `/licenses/FFMPEG-VERSION`; the
upstream copyright file, if shipped by the package, is in
`/licenses/FFMPEG-COPYRIGHT`.

## License

FFmpeg is free software. The jellyfin-ffmpeg build is configured with GPL
components enabled (for example libx264 and libx265) and is therefore
distributed under the terms of the **GNU General Public License** (version 3 or
later as stated in the upstream copyright file). The GPL text is available at
<https://www.gnu.org/licenses/gpl-3.0.html>.

jellyfin-ffmpeg is built and published by the Jellyfin project, not by the
Slouchcat project. Slouchcat does not modify FFmpeg; it runs the unmodified
binaries as separate processes.

Slouchcat itself is licensed under the GNU Affero General Public License,
version 3 (`/licenses/LICENSE`).

## Corresponding source

The complete corresponding source code of jellyfin-ffmpeg, including the build
scripts and patches, is available from the Jellyfin project:

- Repository: <https://github.com/jellyfin/jellyfin-ffmpeg>
- Release for a given version: `https://github.com/jellyfin/jellyfin-ffmpeg/releases/tag/v<version>`,
  where `<version>` is the version in `/licenses/FFMPEG-VERSION` without the
  Debian suite suffix (for example `7.1.4-3-trixie` corresponds to tag `v7.1.4-3`).
- Source packages: the `deb-src` entries of <https://repo.jellyfin.org/debian>.

## Written offer

For at least three years after the last distribution of this image by the
Slouchcat project, the Slouchcat project offers to provide anyone, on request,
a complete machine-readable copy of the corresponding source code of the FFmpeg
build contained in this image, for no more than the cost of physically
performing the distribution. To request it, open an issue in the Slouchcat
repository (<https://github.com/slouchcat/slouchcat/issues>) and state the
image version (`org.opencontainers.image.version` label) and the content of
`/licenses/FFMPEG-VERSION`.

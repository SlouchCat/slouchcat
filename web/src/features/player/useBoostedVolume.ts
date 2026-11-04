import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { supportsWebAudioBoost } from '@/lib/webAudio';
import { BOOST_MAX } from '@/ui/aria';
import { useAudioGraph } from './useAudioGraph';
import { useVolume } from './useVolume';

interface BoostedVolumeOptions {
  /** Shows «Lautstärke 80 %» or «Stumm» over the picture. */
  showOsd: (text: string) => void;
  /** A switch keeps a mute. */
  initialMuted?: boolean;
  /** The level of the previous session of the title (a boost above 100 % is not stored). */
  initialVolume?: number;
  initialNightMode?: boolean;
}

/**
 * The volume of the online and the offline video player (V-3, 11.10): up to 150 % and night mode
 * through Web Audio, except in Safari (11.15). When the browser refuses the graph, night mode and
 * the volume above 100 % go away. `changeVolume` (↑/↓, the wheel) and `toggleMute` (M) say what
 * they did over the picture; the volume control calls `volume` directly.
 */
export function useBoostedVolume(
  media: RefObject<HTMLMediaElement | null>,
  { showOsd, initialMuted, initialVolume, initialNightMode = false }: BoostedVolumeOptions,
) {
  const { t } = useTranslation();
  const [webAudio, setWebAudio] = useState(() => supportsWebAudioBoost());
  const volume = useVolume(media, { initialMuted, initialVolume, max: webAudio ? BOOST_MAX : 1 });
  const [nightMode, setNightMode] = useState(initialNightMode);

  const { setVolume: setLevel, volume: levelBefore } = volume;
  const levelAtFailure = useRef(levelBefore);
  useEffect(() => {
    levelAtFailure.current = levelBefore;
  }, [levelBefore]);
  const audioGraphUnavailable = useCallback(() => {
    setWebAudio(false);
    setNightMode(false);
    if (levelAtFailure.current > 1) setLevel(1);
  }, [setLevel]);
  useAudioGraph({
    media,
    enabled: webAudio,
    nightMode,
    gain: volume.gain,
    onUnavailable: audioGraphUnavailable,
  });

  /** «Lautstärke 80 %»; above 100 % with the warning that it may distort. */
  const volumeText = useCallback(
    (level: number) => {
      const percent = Math.round(level * 100);
      return level > 1
        ? t('player.volumeControl.boosted', { percent })
        : t('player.volume', { percent });
    },
    [t],
  );
  const { step: stepVolume, toggleMute: toggleElementMute, volume: level } = volume;
  const changeVolume = useCallback(
    (delta: number) => showOsd(volumeText(stepVolume(delta))),
    [showOsd, stepVolume, volumeText],
  );
  const toggleMute = useCallback(() => {
    const muted = toggleElementMute();
    showOsd(muted ? t('player.chrome.muted') : volumeText(level));
  }, [level, showOsd, t, toggleElementMute, volumeText]);

  return { volume, webAudio, nightMode, setNightMode, changeVolume, toggleMute };
}

import type { EffectiveLimits, PlaybackLimits } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { RadioGroup, TextField } from '@/ui/aria';

/**
 * Output heights a limit can name, lowest first. The output never exceeds 1080p (AN-17), so the
 * contract's 2160 would be no limit at all and is not offered.
 */
export const limitHeights = [360, 480, 720, 1080] as const;

/** Radio value of "no limit" (an empty radio value would not count as a choice). */
const NO_LIMIT = 'none';

const ranges = {
  maxBitrateKbps: { min: 200, max: 200_000 },
  maxStreams: { min: 1, max: 16 },
  watchLimitMinutes: { min: 1, max: 1440 },
} as const;
type NumberLimit = keyof typeof ranges;

/** The limit fields as the form holds them: texts, empty for "no limit". */
export interface LimitValues {
  maxHeight: string;
  maxBitrateKbps: string;
  maxStreams: string;
  watchLimitMinutes: string;
}

export function limitValues(limits: PlaybackLimits | undefined): LimitValues {
  const text = (value: number | undefined) => (value === undefined ? '' : String(value));
  return {
    maxHeight: text(limits?.maxHeight),
    maxBitrateKbps: text(limits?.maxBitrateKbps),
    maxStreams: text(limits?.maxStreams),
    watchLimitMinutes: text(limits?.watchLimitMinutes),
  };
}

const whole = (value: string) => (/^\d+$/.test(value.trim()) ? Number(value.trim()) : undefined);

/**
 * The `limits` object of a request: every limit with a value; a left-out field clears that limit
 * on the server (the object replaces all limits at once). The server validates the ranges.
 */
export function limitsOf(values: LimitValues, withWatch = true): PlaybackLimits {
  const limits: PlaybackLimits = {};
  const height = limitHeights.find((candidate) => String(candidate) === values.maxHeight);
  if (height) limits.maxHeight = height;
  const bitrate = whole(values.maxBitrateKbps);
  if (bitrate !== undefined) limits.maxBitrateKbps = bitrate;
  const streams = whole(values.maxStreams);
  if (streams !== undefined) limits.maxStreams = streams;
  const watch = whole(values.watchLimitMinutes);
  if (withWatch && watch !== undefined) limits.watchLimitMinutes = watch;
  return limits;
}

/** "bis 720p · 4000 kbit/s · 2 Streams · 60 min pro Tag"; empty without any limit. */
export function limitParts(t: TFunction, limits: EffectiveLimits | PlaybackLimits): string[] {
  const parts: string[] = [];
  if (limits.maxHeight !== undefined) parts.push(t('limits.height', { height: limits.maxHeight }));
  if (limits.maxBitrateKbps !== undefined) {
    parts.push(t('limits.bitrateValue', { value: limits.maxBitrateKbps }));
  }
  if (limits.maxStreams !== undefined) {
    parts.push(t('limits.streamsValue', { count: limits.maxStreams }));
  }
  if (limits.watchLimitMinutes !== undefined) {
    parts.push(t('limits.watchValue', { count: limits.watchLimitMinutes }));
  }
  return parts;
}

/** Whether the account caps a profile below its own limits (a guest account, W-8). */
export function isCapped(own: PlaybackLimits, effective: EffectiveLimits): boolean {
  return (['maxHeight', 'maxBitrateKbps', 'maxStreams'] as const).some(
    (key) => effective[key] !== undefined && effective[key] !== own[key],
  );
}

interface LimitFieldsProps {
  value: LimitValues;
  onChange: (value: LimitValues) => void;
  /** The daily watch time (K-5) is a profile setting; guests have none. */
  withWatch?: boolean;
}

/**
 * The playback limits of W-8 (highest quality, bandwidth, concurrent streams) and K-5 (watch time
 * per day) as form fields; an empty field means no limit.
 */
export function LimitFields({ value, onChange, withWatch = true }: LimitFieldsProps) {
  const { t, i18n } = useTranslation();
  const numbers = new Intl.NumberFormat(i18n.language);
  const range = (key: NumberLimit) => ({
    min: numbers.format(ranges[key].min),
    max: numbers.format(ranges[key].max),
  });
  const set = (key: keyof LimitValues) => (text: string) => onChange({ ...value, [key]: text });
  const validate = (key: NumberLimit) => (text: string) => {
    if (text.trim() === '') return null;
    const number = whole(text);
    const { min, max } = ranges[key];
    return number !== undefined && number >= min && number <= max
      ? null
      : t('limits.range', range(key));
  };
  const numberField = (key: NumberLimit, label: string) => (
    <TextField
      name={`limits.${key}`}
      label={label}
      description={t('limits.hint', range(key))}
      value={value[key]}
      onChange={set(key)}
      inputMode="numeric"
      autoComplete="off"
      validate={validate(key)}
    />
  );
  return (
    <>
      <RadioGroup
        name="limits.maxHeight"
        label={t('limits.maxHeight')}
        value={value.maxHeight || NO_LIMIT}
        onChange={(height) => set('maxHeight')(height === NO_LIMIT ? '' : height)}
        options={[
          { value: NO_LIMIT, label: t('limits.none') },
          ...limitHeights.map((height) => ({
            value: String(height),
            label: t('format.height', { height }),
          })),
        ]}
      />
      {numberField('maxBitrateKbps', t('limits.bitrate'))}
      {numberField('maxStreams', t('limits.streams'))}
      {withWatch && numberField('watchLimitMinutes', t('limits.watch'))}
    </>
  );
}

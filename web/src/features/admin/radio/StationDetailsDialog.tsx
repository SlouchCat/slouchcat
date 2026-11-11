import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { countryName, radioLanguageName, regionName } from '@/app/radio/labels';
import { formatBitrate } from '@/lib/format';
import { FactList, Text } from '@/ui';
import { AriaButton, FormDialog } from '@/ui/aria';
import type { AdminRadioStation } from '../queries';
import styles from './Radio.module.css';

type Source = NonNullable<AdminRadioStation['sources']['name']>;
type Field =
  | 'name'
  | 'country'
  | 'region'
  | 'languages'
  | 'homepage'
  | 'logo'
  | 'provider'
  | 'owner'
  | 'founded'
  | 'wikidataId';

/**
 * A founding date as precise as Wikidata knows it: `1983`, `1983-08` (month and year) or
 * `1983-11-01` (the whole date), in the language of the page.
 */
export function formatFounded(value: string, language: string): string {
  const [year, month, day] = value.split('-').map(Number);
  if (year === undefined || month === undefined) return value;
  const date = new Date(Date.UTC(year, month - 1, day ?? 1));
  return new Intl.DateTimeFormat(
    language,
    day === undefined
      ? { year: 'numeric', month: 'long', timeZone: 'UTC' }
      : { dateStyle: 'long', timeZone: 'UTC' },
  ).format(date);
}

interface StationDetailsDialogProps {
  station: AdminRadioStation;
  onClose: () => void;
}

/**
 * «Angaben» of a station in «Sender» (concept 19 part B, B9.3, stage B2): every value with where
 * it comes from (catalog, own station, by hand, stream, homepage, Wikidata), owner and founding
 * date from Wikidata, then the streams, best first.
 */
export function StationDetailsDialog({ station, onClose }: StationDetailsDialogProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const sources = station.sources;
  const wikidata = station.wikidata;

  // Term and value of each value the station has, the value followed by its origin.
  const facts: [string, ReactNode][] = [];
  const fact = (key: Field, value: ReactNode, source: Source | undefined) => {
    if (!source) return;
    facts.push([
      t(`admin.radio.stations.details.fields.${key}`),
      <span key={key} className={styles.offerStation}>
        <span>{value}</span>
        <span className={styles.muted}>
          {t('admin.radio.stations.details.source', {
            source: t(`admin.radio.stations.details.sources.${source}`),
          })}
        </span>
      </span>,
    ]);
  };
  fact('name', station.name, sources.name);
  if (station.country) fact('country', countryName(language, station.country), sources.country);
  if (station.region) fact('region', regionName(t, station.region), sources.region);
  fact(
    'languages',
    station.languages.map((code) => radioLanguageName(language, code)).join(', '),
    sources.languages,
  );
  if (station.homepage) {
    fact('homepage', <span className={styles.url}>{station.homepage}</span>, sources.homepage);
  }
  fact('logo', t('admin.radio.stations.details.logoPresent'), sources.logo);
  fact('provider', station.provider, sources.provider);
  if (wikidata?.owner) fact('owner', wikidata.owner, 'wikidata');
  if (wikidata?.founded) fact('founded', formatFounded(wikidata.founded, language), 'wikidata');
  if (wikidata) fact('wikidataId', <span className={styles.url}>{wikidata.id}</span>, 'wikidata');

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.radio.stations.details.title', { name: station.name })}
      initialFocus="[data-details-close]"
    >
      <div className={styles.form}>
        <FactList facts={facts} />
        {!wikidata && (
          <Text tone="secondary">
            {t(
              station.offered
                ? 'admin.radio.stations.details.noWikidata'
                : 'admin.radio.stations.details.wikidataOfferOnly',
            )}
          </Text>
        )}

        <h3 className={styles.summaryTitle}>{t('admin.radio.stations.details.streams')}</h3>
        {station.streams.length === 0 ? (
          <Text tone="secondary">{t('admin.radio.stations.streamsEmpty')}</Text>
        ) : (
          <ul className={styles.listeners}>
            {station.streams.map((stream) => (
              <li key={stream.id} className={styles.offerStation}>
                <span className={styles.url}>{stream.urlResolved ?? stream.url}</span>
                <span className={styles.muted}>
                  {[
                    stream.codec,
                    stream.bitrateKbps !== undefined &&
                      formatBitrate(stream.bitrateKbps * 1000, language),
                    stream.hls && t('admin.radio.stations.streamHls'),
                    stream.preferred && t('admin.radio.stations.streamPreferred'),
                    !stream.reachable && t('admin.radio.stations.streamUnreachable'),
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </span>
              </li>
            ))}
          </ul>
        )}
        <div className={styles.dialogActions}>
          <AriaButton data-details-close="" onPress={onClose}>
            {t('admin.radio.stations.details.close')}
          </AriaButton>
        </div>
      </div>
    </FormDialog>
  );
}

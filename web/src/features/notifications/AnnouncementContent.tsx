import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatDate } from '@/lib/format';
import { MessageContent } from '@/ui/aria';

/** What a profile reads of an announcement. */
export interface AnnouncementText {
  title: string;
  body: string;
  publishedAt: string;
}

/**
 * The texts of the announcement dialog (A-7): the kicker «Ankündigung», the title, the date of
 * publication and the text. The dialog of the profiles and the preview on «Ankündigungen» take
 * them from here, so both say the same.
 */
export function announcementMessage(t: TFunction, language: string, text: AnnouncementText) {
  return {
    kicker: t('announcements.kicker'),
    title: text.title,
    meta: t('announcements.published', { date: formatDate(text.publishedAt, language) }),
    body: text.body,
  };
}

/**
 * The content of the announcement dialog without its buttons, for the preview «So sehen es die
 * Profile» of the administration (concept 12, 11.3). Its title is plain text there, under the
 * page's own headings.
 */
export function AnnouncementContent(text: AnnouncementText) {
  const { t, i18n } = useTranslation();
  return <MessageContent {...announcementMessage(t, i18n.language, text)} preview />;
}

import type { ProfileAnnouncement } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatDate } from '@/lib/format';
import { Notice } from '@/ui';
import { MessageDialog } from '@/ui/aria';
import { ackAnnouncement, announcementsQuery } from './announcements';

interface AnnouncementDialogProps {
  profileId: number;
  /** The open announcements, newest first; the first one shows. */
  announcements: ProfileAnnouncement[];
  /** Closed without acknowledging. */
  onClose: () => void;
}

/** The newest open announcement; "Verstanden" acknowledges it for the profile (A-7). */
export function AnnouncementDialog({ profileId, announcements, onClose }: AnnouncementDialogProps) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  // How many there were at the start, for "2 von 3".
  const [total] = useState(announcements.length);
  const drop = (id: number) =>
    queryClient.setQueryData<ProfileAnnouncement[]>(announcementsQuery(profileId).queryKey, (old) =>
      old?.filter((announcement) => announcement.id !== id),
    );
  const ack = useMutation({
    mutationFn: ackAnnouncement,
    onSuccess: (_result, id) => drop(id),
    // Withdrawn meanwhile: nothing left to acknowledge.
    onError: (error, id) => {
      if (isApiError(error, 404)) drop(id);
    },
  });
  const [current] = announcements;
  if (!current) return null;
  const failed = ack.isError && !isApiError(ack.error, 404);

  return (
    <MessageDialog
      isOpen
      onDismiss={onClose}
      kicker={t('announcements.kicker')}
      title={current.title}
      meta={t('announcements.published', {
        date: formatDate(current.publishedAt, i18n.language),
      })}
      body={current.body}
      status={
        total > 1
          ? t('announcements.counter', { index: total - announcements.length + 1, count: total })
          : undefined
      }
      error={failed ? <Notice tone="danger">{problemMessage(t, ack.error)}</Notice> : undefined}
      dismissLabel={t('announcements.later')}
      confirmLabel={t('announcements.acknowledge')}
      isPending={ack.isPending}
      onConfirm={() => {
        if (!ack.isPending) ack.mutate(current.id);
      }}
    />
  );
}

import type { ChannelSetting } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, Notice, Text, TextLink } from '@/ui';
import { AriaButton, SettingRow } from '@/ui/aria';
import { createTelegramLink, deleteTelegramLink } from '../queries';
import {
  afterDialogs,
  afterRender,
  LINK_POLL_MS,
  nextMessage,
  Status,
  useRefreshChannels,
  useStoreChannel,
  type Ask,
  type Message,
} from './shared';
import styles from './channels.module.css';

interface TelegramLinkProps {
  setting: ChannelSetting;
  /** The page asks before the link goes (one dialog for the page). */
  ask: Ask;
  /** The page asks for the open code every few seconds; false while it holds its queries. */
  polling: boolean;
  /** The focus fell to the body (a button went with a change): the h1 takes it. */
  focusTitleIfLost: () => void;
  /** The h1 takes the focus. */
  focusTitle: () => void;
}

/**
 * Telegram (BEN-2): the link state, the link code with the instruction for the bot, renew and
 * unlink. Unlinking, and dropping an open code, asks first (concept 12, 11.5 point 5). The code
 * comes from a transient mutation, so it never stays in the mutation cache.
 */
export function TelegramLink({
  setting,
  ask,
  polling,
  focusTitleIfLost,
  focusTitle,
}: TelegramLinkProps) {
  const { t, i18n } = useTranslation();
  const blockRef = useRef<HTMLDivElement>(null);
  const store = useStoreChannel();
  const refresh = useRefreshChannels();
  const [message, setMessage] = useState<Message | null>(null);
  // The error of the last action; a new action clears it (link and unlink alike).
  const [error, setError] = useState<unknown>(null);
  const telegram = setting.telegram;
  const linked = telegram?.linked ?? false;
  const link = useTransientMutation(createTelegramLink, (next) => {
    store(next);
  });
  const unlink = useMutation({
    mutationFn: deleteTelegramLink,
    onError: setError,
    onSuccess: async () => {
      setMessage((current) => nextMessage(current, t('settings.notifications.telegram.unlinked')));
      await refresh();
      // The button that asked went with the link. Once the dialog has closed (its focus restore
      // aims at that button), the h1 takes the focus if it was lost or is still in this block.
      await afterDialogs();
      const active = document.activeElement;
      if (!active || active === document.body || blockRef.current?.contains(active)) focusTitle();
    },
  });
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setMessage(null);
    setError(null);
  };
  // Empty when the server could not ask Telegram for the bot's name (getMe failed).
  const bot = telegram?.botUsername ?? '';
  const code = linked ? undefined : telegram?.linkCode;

  const linkedText = telegram?.chatId
    ? t('settings.notifications.telegram.linkedChat', { chat: telegram.chatId })
    : t('settings.notifications.telegram.linked');
  // The bot got the code (the poll brought it): the status line says so, and «Code erneuern» is
  // gone, perhaps with the focus.
  const wasLinked = useRef(linked);
  useEffect(() => {
    if (linked && !wasLinked.current) {
      setMessage((current) => nextMessage(current, linkedText));
      void afterRender().then(focusTitleIfLost);
    }
    wasLinked.current = linked;
  }, [linked, linkedText, focusTitleIfLost]);

  const askUnlink = () =>
    ask({
      title: linked
        ? t('settings.notifications.telegram.unlinkConfirm.title')
        : t('settings.notifications.telegram.discardConfirm.title'),
      body: linked
        ? t('settings.notifications.telegram.unlinkConfirm.body')
        : t('settings.notifications.telegram.discardConfirm.body'),
      confirmLabel: linked
        ? t('settings.notifications.telegram.unlink')
        : t('settings.notifications.telegram.discardConfirm.confirm'),
      tone: 'danger',
      onConfirm: () => {
        start();
        unlink.mutate();
      },
    });

  return (
    <div ref={blockRef} className={styles.telegram}>
      <SettingRow
        label={t('settings.notifications.telegram.label')}
        help={t('settings.notifications.telegram.help')}
      >
        {linked ? (
          <Chip tone="success" icon="check">
            {linkedText}
          </Chip>
        ) : (
          <Chip tone="outline">{t('settings.notifications.telegram.notLinked')}</Chip>
        )}
        {!linked && (
          <AriaButton
            isPending={link.isPending}
            onPress={() => {
              start();
              void link.run(undefined).then((outcome) => {
                if (!outcome.ok) setError(outcome.error);
              });
            }}
          >
            {code
              ? t('settings.notifications.telegram.renew')
              : t('settings.notifications.telegram.link')}
          </AriaButton>
        )}
        {(linked || code) && (
          <AriaButton variant="danger" isPending={unlink.isPending} onPress={askUnlink}>
            {t('settings.notifications.telegram.unlink')}
          </AriaButton>
        )}
      </SettingRow>
      {code && (
        <div className={styles.code}>
          <p className={styles.instruction}>
            {bot
              ? t('settings.notifications.telegram.instruction', { bot, code })
              : t('settings.notifications.telegram.instructionNoBot', { code })}
          </p>
          {!bot && (
            <Text tone="secondary" variant="caption">
              {t('settings.notifications.telegram.noBot')}
            </Text>
          )}
          {telegram?.linkExpiresAt && (
            <Text tone="muted" variant="caption">
              {t('settings.notifications.telegram.expires', {
                time: formatDateTime(telegram.linkExpiresAt, i18n.language),
              })}
            </Text>
          )}
          <Text tone="muted" variant="caption">
            {polling
              ? t('settings.notifications.telegram.waiting', { count: LINK_POLL_MS / 1000 })
              : t('settings.notifications.telegram.paused')}
          </Text>
          {bot && (
            <TextLink
              href={`https://t.me/${encodeURIComponent(bot)}?start=${encodeURIComponent(code)}`}
              external
            >
              {t('settings.notifications.telegram.open')}
            </TextLink>
          )}
        </div>
      )}
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
      <Status message={message} />
    </div>
  );
}

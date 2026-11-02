import { useQuery } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { Notice, Panel, Text } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { RESTART_GIVE_UP_MS, RESTART_POLL_MS, restartedSince } from './restart';
import data from './RestartPanel.module.css';

export interface RestartPanelProps {
  /** When the 202 of the restore or the restart arrived (ms, `Date.now()`). */
  acceptedAt: number;
  title: string;
  body: string;
  /** An optional warning below the body (a restore without secrets). */
  notice?: ReactNode;
  /** Runs once the new server process answers: the sign-in after a restore, a reload after a restart. */
  onBack: () => void;
}

/**
 * «Server startet neu» after a restore or a restart (concept 11): asks `/healthz` every 2 s and
 * calls `onBack` as soon as the new server process answers. After 90 s without it the panel stops
 * asking and says so, with a button that waits again. The caller passes its title, body and
 * notice; the waiting texts are the same for both.
 */
export function RestartPanel({ acceptedAt, title, body, notice, onBack }: RestartPanelProps) {
  const { t } = useTranslation();
  const heading = useRef<HTMLHeadingElement>(null);
  const headingId = useId();
  // Start of the current wait: the restore or restart, or the last «Erneut prüfen».
  const [waitingSince, setWaitingSince] = useState(acceptedAt);
  const [gaveUp, setGaveUp] = useState(false);
  const health = useQuery({
    queryKey: ['restart', 'health', acceptedAt],
    queryFn: ({ signal }) => request(api.GET('/healthz', { signal })),
    refetchInterval: gaveUp ? false : RESTART_POLL_MS,
    retry: false,
    gcTime: 0,
  });
  const back =
    health.data !== undefined && restartedSince(health.data, acceptedAt, health.dataUpdatedAt);

  // The page content changed under the confirmation that started this: the heading takes the
  // focus, so keyboard and screen reader users land on the news.
  useEffect(() => {
    heading.current?.focus();
  }, []);
  // onBack is read through a ref, so a new function from a re-render does not call it twice.
  const onBackRef = useRef(onBack);
  useEffect(() => {
    onBackRef.current = onBack;
  }, [onBack]);
  useEffect(() => {
    if (back) onBackRef.current();
  }, [back]);
  useEffect(() => {
    if (back || gaveUp) return;
    const timer = setTimeout(
      () => setGaveUp(true),
      Math.max(0, waitingSince + RESTART_GIVE_UP_MS - Date.now()),
    );
    return () => clearTimeout(timer);
  }, [back, gaveUp, waitingSince]);

  const waitAgain = () => {
    setGaveUp(false);
    setWaitingSince(Date.now());
    void health.refetch();
    // The button goes away with the notice: the focus goes back to the heading.
    heading.current?.focus();
  };

  return (
    <Panel as="section" aria-labelledby={headingId}>
      <h2 id={headingId} ref={heading} tabIndex={-1} className={data.panelTitle}>
        {title}
      </h2>
      <Text>{body}</Text>
      {notice}
      <p role="status" className={data.status}>
        {back
          ? t('admin.restart.back')
          : gaveUp
            ? t('admin.restart.silent', { seconds: RESTART_GIVE_UP_MS / 1000 })
            : t('admin.restart.waiting')}
      </p>
      {gaveUp && !back && (
        <Notice
          tone="warning"
          live={false}
          title={t('admin.restart.slow.title')}
          actions={
            <AriaButton icon="refresh" onPress={waitAgain}>
              {t('admin.restart.slow.retry')}
            </AriaButton>
          }
        >
          {t('admin.restart.slow.body')}
        </Notice>
      )}
    </Panel>
  );
}

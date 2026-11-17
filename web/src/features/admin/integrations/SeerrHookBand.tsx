import type { IntegrationCreated } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, FactList, Notice, Skeleton, Text } from '@/ui';
import { afterDialogsClosed, AriaButton, CodeBlock, DisclosureGroup, focusLost } from '@/ui/aria';
import { CreatedAddress, EventLog, RowActions } from '../WebhooksSection';
import {
  createIntegration,
  dashboardQuery,
  integrationsQuery,
  seerrWebhookQuery,
} from '../queries';
import styles from './Integrations.module.css';

/**
 * The band «Sofortmeldung» of Integrationen › Seerr (concept 25 stage 4, F-18, administration
 * 4.3): Seerr's webhook only triggers the sync of the wishes. «Adresse erzeugen» creates the
 * webhook integration of the kind `seerr` (the same one «Webhooks von Radarr und Sonarr» lists),
 * whose address shows once; the JSON payload and the notification types to enter in Seerr, each
 * with a copy button; renewing and deleting as for every webhook integration, and its reports
 * (`sync_queued`). Closed at first, open while an address was just made.
 */
export function SeerrHookBand() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const integrations = useQuery(integrationsQuery);
  const [open, setOpen] = useState(false);
  // Read once the band opens; the template's «{{…}}» are Seerr's and only show while open.
  const setup = useQuery({ ...seerrWebhookQuery, enabled: open });
  const [created, setCreated] = useState<IntegrationCreated | null>(null);
  const [error, setError] = useState<unknown>(null);
  const createRef = useRef<HTMLButtonElement>(null);
  const hook = integrations.data?.find((integration) => integration.kind === 'seerr');

  const shown = (next: IntegrationCreated) => {
    setCreated(next);
    // Something just created stays open (design rule 1.3).
    setOpen(true);
  };
  // The answer carries the secret address: nothing of it stays in the mutation cache.
  const create = useTransientMutation(createIntegration, async (next: IntegrationCreated) => {
    shown(next);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: integrationsQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
  });

  const language = intlLocale(i18n.language);
  return (
    <DisclosureGroup
      title={t('admin.integrations.seerr.hook.title')}
      isExpanded={open}
      onExpandedChange={setOpen}
      meta={
        integrations.data && (
          <Chip tone={hook ? 'success' : 'outline'}>
            {t(hook ? 'admin.integrations.seerr.hook.on' : 'admin.integrations.seerr.hook.off')}
          </Chip>
        )
      }
    >
      <div className={styles.stack}>
        <Text tone="secondary">{t('admin.integrations.seerr.hook.intro')}</Text>
        {created && (
          <CreatedAddress key={created.url} created={created} onDismiss={() => setCreated(null)} />
        )}
        {integrations.isPending && <Skeleton shape="block" />}
        {integrations.isError && (
          <Notice tone="danger">{problemMessage(t, integrations.error)}</Notice>
        )}
        {integrations.data && !hook && (
          <div className={styles.actions}>
            <AriaButton
              ref={createRef}
              variant="primary"
              icon="plus"
              isPending={create.isPending}
              onPress={() => {
                setError(null);
                void create.run({ name: t('admin.integrations.seerr.hook.name'), kind: 'seerr' });
              }}
            >
              {t('admin.integrations.seerr.hook.create')}
            </AriaButton>
          </div>
        )}
        {create.error ? <Notice tone="danger">{problemMessage(t, create.error)}</Notice> : null}
        {hook && (
          <>
            <FactList
              facts={[
                [
                  t('admin.integrations.seerr.hook.address'),
                  <code key="address" className={styles.url}>
                    {t('admin.integrations.webhooks.tail', { tail: hook.tokenTail })}
                  </code>,
                ],
                [
                  t('admin.integrations.seerr.hook.last'),
                  hook.lastSeenAt
                    ? formatDateTime(hook.lastSeenAt, language)
                    : t('admin.integrations.seerr.hook.never'),
                ],
              ]}
            />
            <RowActions
              integration={hook}
              onRenewed={shown}
              onRemoved={() => {
                setCreated(null);
                // The buttons went with the integration: the focus goes to «Adresse erzeugen».
                void afterDialogsClosed().then(() => {
                  if (focusLost()) createRef.current?.focus();
                });
              }}
              onError={setError}
            />
          </>
        )}
        {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
        <Notice tone="warning" live={false}>
          {t('admin.integrations.seerr.hook.single')}
        </Notice>
        {open && setup.isPending && <Skeleton shape="block" />}
        {setup.isError && <Notice tone="danger">{problemMessage(t, setup.error)}</Notice>}
        {open && setup.data && (
          <>
            <Text>{t('admin.integrations.seerr.hook.where')}</Text>
            <CodeBlock
              code={setup.data.jsonPayload}
              label={t('admin.integrations.seerr.hook.payload')}
              copyLabel={t('admin.integrations.seerr.hook.copyPayload')}
              copiedMessage={t('admin.integrations.seerr.hook.copied')}
              copyFailedMessage={t('admin.integrations.seerr.hook.copyFailed')}
            />
            <Text>{t('admin.integrations.seerr.hook.typesIntro')}</Text>
            <CodeBlock
              code={setup.data.typeNames.join('\n')}
              label={t('admin.integrations.seerr.hook.types')}
              copyLabel={t('admin.integrations.seerr.hook.copyTypes')}
              copiedMessage={t('admin.integrations.seerr.hook.copied')}
              copyFailedMessage={t('admin.integrations.seerr.hook.copyFailed')}
            />
          </>
        )}
        {hook && (
          <section className={styles.stack} aria-labelledby="seerr-hook-log">
            <h4 id="seerr-hook-log" className={styles.itemTitle}>
              {t('admin.integrations.seerr.hook.log')}
            </h4>
            <EventLog integration={hook} />
          </section>
        )}
      </div>
    </DisclosureGroup>
  );
}

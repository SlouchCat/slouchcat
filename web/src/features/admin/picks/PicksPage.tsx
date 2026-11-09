import type { AdminUserPick, PicksSettings } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useRef, useState, type FormEvent } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Icon, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  Form,
  SettingRow,
  StatusRegion,
  TextField,
  afterDialogsClosed,
  focusLost,
  focusTitleIfLost,
  useStatusMessage,
} from '@/ui/aria';
import { picksQuery, picksSettingsQuery, removeUserPick, updatePicksSettings } from '../queries';
import { pickColumns } from './columns';
import styles from './Picks.module.css';

/**
 * «User-Empfehlungen» (concept 18 part B, 14.7, ADR 0028): how long a title stays after its newest
 * pick, and the titles the profiles of the owner's account recommend now, newest first, with the
 * day of the newest pick, the days left and the number of picks, never a profile. «Entfernen»
 * removes every pick of a title. The profiles recommend on the title page (the star); the
 * administration does not add titles.
 */
export function PicksPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.picks.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.picks.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <PicksContent />;
}

function PicksContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  /** The pick «Entfernen» asks about and the button that asked (the focus goes back there). */
  const [asking, setAsking] = useState<{ pick: AdminUserPick; from: HTMLElement | null } | null>(
    null,
  );
  const picks = useQuery({ ...picksQuery, refetchOnWindowFocus: asking === null });
  const settings = useQuery(picksSettingsQuery);

  // The start page, the viewer's page of picks and the side navigation's count show them.
  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['admin', 'picks'] }),
      queryClient.invalidateQueries({ queryKey: ['home'] }),
      queryClient.invalidateQueries({ queryKey: ['picks'] }),
    ]);

  const remove = useMutation({
    mutationFn: (pick: AdminUserPick) => removeUserPick(pick.itemId),
    onSuccess: async (_data, pick) => {
      setFailure(null);
      setMessage(t('admin.picks.remove.done', { title: pick.title }));
      await invalidate();
      // The row and its button are gone: once React rendered that, the h1 takes the focus.
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    onError: (error) => {
      setMessage(null);
      setFailure(error);
    },
  });

  const onRemove = useCallback((pick: AdminUserPick) => {
    const from = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setAsking({ pick, from });
  }, []);
  const onConfirmOpenChange = (open: boolean) => {
    if (open) return;
    const button = asking?.from;
    setAsking(null);
    // Back to the button that asked; it may be gone by now (its row went), then the h1.
    void afterDialogsClosed().then(() => {
      if (focusLost() && button?.isConnected) button.focus();
      else focusTitleIfLost(titleRef);
    });
  };

  const columns = useMemo(
    () => pickColumns(t, i18n.language, onRemove),
    [t, i18n.language, onRemove],
  );
  const list = picks.data ?? [];

  return (
    <AdminPage title={t('admin.picks.title')} help={t('admin.picks.intro')} titleRef={titleRef}>
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />

      <BandPanel title={t('admin.picks.retention.title')} icon="settings">
        {settings.isPending && <Skeleton shape="block" />}
        {settings.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, settings.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {settings.data && (
          <RetentionForm
            stored={settings.data}
            onSaved={async (saved) => {
              setFailure(null);
              setMessage(t('admin.picks.retention.saved', { count: saved.retentionDays }));
              queryClient.setQueryData(picksSettingsQuery.queryKey, saved);
              await invalidate();
            }}
          />
        )}
      </BandPanel>

      <BandPanel
        title={t('admin.picks.list')}
        icon="star"
        meta={
          picks.data
            ? t('admin.picks.count', {
                count: list.length,
                number: new Intl.NumberFormat(i18n.language).format(list.length),
              })
            : undefined
        }
      >
        {picks.isPending && <Skeleton shape="block" />}
        {picks.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, picks.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void picks.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {picks.data?.length === 0 && <Text tone="secondary">{t('admin.picks.empty')}</Text>}
        {list.length > 0 && (
          <DataTable
            caption={t('admin.picks.list')}
            columns={columns}
            rows={list}
            rowKey={(pick) => pick.itemId}
            layout="fixed"
          />
        )}
        <Text tone="secondary">{t('admin.picks.anonymous')}</Text>
      </BandPanel>

      <p className={styles.hint}>
        <Icon name="info" size="sm" />
        <span>
          {/* Not <link>: the Trans parser takes it for the void HTML element. */}
          <Trans
            i18nKey="admin.picks.homeRowsHint"
            components={{ page: <TextLink href={paths.adminHomeRows}>{null}</TextLink> }}
          />
        </span>
      </p>
      {asking && (
        <ConfirmDialog
          isOpen
          onOpenChange={onConfirmOpenChange}
          title={t('admin.picks.remove.title', { title: asking.pick.title })}
          body={t('admin.picks.remove.body')}
          confirmLabel={t('admin.picks.remove.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => remove.mutate(asking.pick)}
        />
      )}
    </AdminPage>
  );
}

/**
 * «Empfehlungen bleiben … Tage in der Liste» with «Speichern». The server checks the range (7 to
 * 365, rule 12); its refusal shows at the field.
 */
function RetentionForm({
  stored,
  onSaved,
}: {
  stored: PicksSettings;
  onSaved: (saved: PicksSettings) => Promise<unknown>;
}) {
  const { t } = useTranslation();
  const [days, setDays] = useState(() => String(stored.retentionDays));
  const save = useMutation({
    mutationFn: (body: PicksSettings) => updatePicksSettings(body),
    onSuccess: async (saved) => {
      setDays(String(saved.retentionDays));
      await onSaved(saved);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = Number(days.trim());
    // Not a whole number: the server would refuse it the same way.
    save.mutate({ retentionDays: Number.isInteger(value) ? value : 0 });
  };
  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      <SettingRow label={t('admin.picks.retention.label')} help={t('admin.picks.retention.help')}>
        <div className={styles.retention}>
          <TextField
            name="retentionDays"
            type="number"
            inputMode="numeric"
            label={t('admin.picks.retention.label')}
            hideLabel
            isRequired
            value={days}
            onChange={setDays}
          />
          <span className={styles.unit}>{t('admin.picks.retention.unit')}</span>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </SettingRow>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
    </Form>
  );
}

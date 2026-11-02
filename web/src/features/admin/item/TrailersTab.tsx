import type { AdminItem, Trailer, TrailerRequest } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Chip, Notice, Text } from '@/ui';
import {
  AriaButton,
  DataTable,
  Form,
  FormDialog,
  RadioGroup,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import styles from './AdminItem.module.css';

/** A bare YouTube video id; anything else goes to the server as a URL. */
const YOUTUBE_ID = /^[A-Za-z0-9_-]{8,16}$/;
const LANG_CODE = /^[a-z]{2,3}$/;

type ItemTrailer = AdminItem['trailers'][number];

function TrailerForm({
  itemId,
  onClose,
  onAdded,
}: {
  itemId: string;
  onClose: () => void;
  onAdded: () => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const [lang, setLang] = useState(i18n.language === 'en' ? 'en' : 'de');
  const add = useMutation({
    mutationFn: (body: TrailerRequest): Promise<Trailer> =>
      request(api.POST('/admin/items/{id}/trailers', { params: { path: { id: itemId } }, body })),
    onSuccess: async () => {
      await onAdded();
      onClose();
    },
  });
  const errors = add.error
    ? formErrors(t, add.error, { youtubeId: 'source', url: 'source', lang: 'langCode' })
    : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const source = formText(data, 'source');
    const code = lang === 'other' ? formText(data, 'langCode') : lang;
    add.mutate({
      lang: code,
      ...(YOUTUBE_ID.test(source) ? { youtubeId: source } : { url: source }),
    });
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <TextField
        name="source"
        label={t('admin.item.trailers.url')}
        description={t('admin.item.trailers.urlHint')}
        autoComplete="off"
        spellCheck="false"
        mono
        isRequired
        validate={(value) => (value.trim() ? null : t('errors.field.required'))}
      />
      <RadioGroup
        label={t('admin.item.trailers.lang')}
        value={lang}
        onChange={setLang}
        options={[
          { value: 'de', label: t('language.de') },
          { value: 'en', label: t('language.en') },
          { value: 'other', label: t('admin.item.trailers.langOther') },
        ]}
      />
      {lang === 'other' && (
        <TextField
          name="langCode"
          label={t('admin.item.trailers.langCode')}
          description={t('admin.item.trailers.langCodeHint')}
          autoComplete="off"
          isRequired
          validate={(value) => (LANG_CODE.test(value.trim()) ? null : t('errors.field.invalid'))}
        />
      )}
      <div className={styles.dialogActions}>
        <AriaButton variant="ghost" onPress={onClose}>
          {t('common.cancel')}
        </AriaButton>
        <AriaButton type="submit" variant="primary" isPending={add.isPending}>
          {add.isPending ? t('admin.item.trailers.submitting') : t('admin.item.trailers.submit')}
        </AriaButton>
      </div>
    </Form>
  );
}

/**
 * The tab «Trailer» (TIT-7): the search state, the trailers with language, source and «Offiziell»,
 * «Hinzufügen» in a dialog. «Löschen» asks through the page's own confirmation, so a row that goes
 * meanwhile cannot take the question with it.
 */
export function TrailersTab({
  item,
  onAdded,
  onDelete,
}: {
  item: AdminItem;
  onAdded: () => Promise<void>;
  onDelete: (trailer: ItemTrailer) => void;
}) {
  const { t } = useTranslation();
  const [adding, setAdding] = useState(false);
  const labelId = (trailer: ItemTrailer) => `admin-item-trailer-${trailer.id}`;
  const columns: DataColumn<ItemTrailer>[] = [
    {
      key: 'video',
      header: t('admin.item.trailers.columns.video'),
      rowHeader: true,
      cell: (trailer) => (
        <span id={labelId(trailer)} className={styles.chips}>
          <code className={styles.path}>{trailer.youtubeId}</code>
          {trailer.title && <span>{trailer.title}</span>}
        </span>
      ),
    },
    {
      key: 'lang',
      header: t('admin.item.trailers.lang'),
      cell: (trailer) => <Chip tone="outline">{trailer.lang.toUpperCase()}</Chip>,
    },
    {
      key: 'source',
      header: t('admin.item.trailers.columns.source'),
      cell: (trailer) => (
        <span className={styles.chips}>
          <span className={styles.muted}>{t(`admin.item.trailers.source.${trailer.source}`)}</span>
          {trailer.official && <Chip tone="info">{t('admin.item.trailers.official')}</Chip>}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('admin.item.trailers.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (trailer) => (
        <AriaButton
          variant="danger"
          aria-describedby={labelId(trailer)}
          onPress={() => onDelete(trailer)}
        >
          {t('admin.item.trailers.delete.action')}
        </AriaButton>
      ),
    },
  ];
  return (
    <section className={styles.tabBody} aria-labelledby="admin-item-trailers">
      <div className={styles.sectionHead}>
        <h2 id="admin-item-trailers" className={styles.sectionTitle}>
          {t('admin.item.trailers.title')}
        </h2>
        <Chip tone="info">{t(`admin.item.trailers.state.${item.trailerState}`)}</Chip>
        <span className={styles.headAction}>
          <AriaButton icon="plus" onPress={() => setAdding(true)}>
            {t('admin.item.trailers.add')}
          </AriaButton>
        </span>
      </div>
      {item.trailers.length === 0 ? (
        <Text tone="secondary">{t('admin.item.trailers.empty')}</Text>
      ) : (
        <DataTable
          caption={t('admin.item.trailers.title')}
          columns={columns}
          rows={item.trailers}
          rowKey={(trailer) => String(trailer.id)}
        />
      )}
      {adding && (
        <FormDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) setAdding(false);
          }}
          title={t('admin.item.trailers.addTitle')}
        >
          <TrailerForm itemId={item.id} onClose={() => setAdding(false)} onAdded={onAdded} />
        </FormDialog>
      )}
    </section>
  );
}

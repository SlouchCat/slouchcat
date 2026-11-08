import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useMemo, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { facetValueName } from '@/app/radio/labels';
import { radioQueryParams } from '@/features/radio/filters';
import { radioFacetsQuery } from '@/features/radio/queries';
import { Notice, SelectField, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  CoverageBars,
  DataTable,
  SectionHeader,
  StatusRegion,
  TextField,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  useStatusMessage,
  type CoverageRow,
  type DataColumn,
} from '@/ui/aria';
import {
  radioAdminKeys,
  radioTagRulesQuery,
  setRadioTagRules,
  type RadioTagRule,
  type RadioTagRuleView,
  type RadioTagRules,
} from '../queries';
import {
  facetGroup,
  facetLabel,
  facetValues,
  parseTarget,
  RULE_FACETS,
  targetOf,
  type RuleFacet,
} from './facets';
import styles from './Radio.module.css';

/** Rows of the worklist and of the rules table shown at first; «Weitere» adds as many. */
const PAGE = 50;

type WorkTag = RadioTagRules['worklist'][number];

/** The administration's rules as the server holds them: the body of the next PUT. */
const adminRules = (data: RadioTagRules): RadioTagRule[] =>
  data.rules
    .filter((rule) => rule.source === 'admin')
    .map(({ tag, facet, value }) => ({ tag, facet, value: value ?? null }));

/** The list with `rule` in place of the administration's rule for its tag and facet (D-2). */
const withRule = (list: readonly RadioTagRule[], rule: RadioTagRule) => [
  ...list.filter((entry) => !(entry.tag === rule.tag && entry.facet === rule.facet)),
  rule,
];

const withoutRule = (list: readonly RadioTagRule[], rule: RadioTagRuleView) =>
  list.filter((entry) => !(entry.tag === rule.tag && entry.facet === rule.facet));

/** The coverage rows of «Übersicht» and «Einordnung» (D-4). */
export function coverageRows(
  t: TFunction,
  language: string,
  coverage: RadioTagRules['coverage'],
): CoverageRow[] {
  const percent = new Intl.NumberFormat(language, { style: 'percent', maximumFractionDigits: 0 });
  const number = new Intl.NumberFormat(language);
  return coverage.map((entry) => ({
    key: entry.facet,
    label: facetLabel(t, entry.facet),
    assigned: entry.assigned,
    total: entry.total,
    percentText: percent.format(entry.total > 0 ? entry.assigned / entry.total : 0),
    countText: t('admin.radio.rules.coverageCount', {
      assigned: number.format(entry.assigned),
      total: number.format(entry.total),
    }),
  }));
}

interface TagRulesTabProps {
  /** The page's h1: it takes the focus when the row that had it went away. */
  titleRef: RefObject<HTMLHeadingElement | null>;
}

/** The key of a rule in the table and in the drafts: tag and facet (D-2). */
const ruleKey = (rule: { tag: string; facet: string }) => `${rule.tag}\n${rule.facet}`;

/** A save: the whole list, the message after it, and the row it came from. */
interface Save {
  list: RadioTagRule[];
  done: string;
  /** The worklist tag or the rule key (ruleKey) whose button saves. */
  row: string;
}

/**
 * «Einordnung» (concept 16, 5.4 and 11; D-1 to D-4): the coverage per facet, the worklist of the
 * tags no rule maps (most stations first; a choice and «Zuordnen» add a rule, «Ignorieren» takes
 * the tag off the list) and the effective rules, defaults and the administration's, whose value
 * changes with a choice and «Übernehmen»; the administration's own rules can be removed after a
 * confirmation. Every change sends the administration's whole list, built from the last answer,
 * and applies at once; while one is on its way every other save waits (no PUT overtakes another
 * and loses its rule).
 */
export function TagRulesTab({ titleRef }: TagRulesTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const rules = useQuery(radioTagRulesQuery);
  // Genres the catalog uses beyond the known keys (a newer default of the server).
  const facets = useQuery(radioFacetsQuery(radioQueryParams({})));

  // Worklist choices by tag, rule drafts by ruleKey; a saved row drops its own.
  const [choices, setChoices] = useState<Record<string, string>>({});
  const [ruleChoices, setRuleChoices] = useState<Record<string, string>>({});
  const [workShown, setWorkShown] = useState(PAGE);
  const [rulesShown, setRulesShown] = useState(PAGE);
  const [query, setQuery] = useState('');
  const [removing, setRemoving] = useState<RadioTagRuleView | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const save = useMutation({
    mutationFn: ({ list }: Save) => setRadioTagRules(list),
    onSuccess: async (next, { done, row }) => {
      queryClient.setQueryData(radioTagRulesQuery.queryKey, next);
      const drop = (before: Record<string, string>) =>
        Object.fromEntries(Object.entries(before).filter(([key]) => key !== row));
      setChoices(drop);
      setRuleChoices(drop);
      setMessage(done);
      // The stations' facets changed with the rules: their lists and counts follow.
      await queryClient.invalidateQueries({
        predicate: (query) =>
          query.queryKey[0] === 'radio' ||
          (query.queryKey[0] === radioAdminKeys.all[0] &&
            query.queryKey[1] === radioAdminKeys.all[1] &&
            query.queryKey[2] !== 'tagRules'),
      });
      await afterRender();
      focusTitleIfLost(titleRef);
    },
  });

  const data = rules.data;
  const values = useMemo(() => {
    const used = (facet: RuleFacet) =>
      (data?.rules ?? [])
        .filter((rule) => rule.facet === facet && rule.value)
        .map((rule) => rule.value ?? '');
    return {
      genre: facetValues('genre', [
        ...(facets.data?.genre ?? []).map((entry) => entry.value),
        ...used('genre'),
      ]),
      kind: facetValues('kind', used('kind')),
      mood: facetValues('mood', used('mood')),
    } satisfies Record<RuleFacet, string[]>;
  }, [data, facets.data]);

  if (rules.isPending) return <Skeleton shape="block" />;
  if (rules.isError || !data) {
    return <Notice tone="danger" title={problemMessage(t, rules.error)} />;
  }

  const current = adminRules(data);
  const busy = save.isPending;
  const saving = (row: string) => busy && save.variables.row === row;
  const assign = (tag: string) => {
    const target = parseTarget(choices[tag] ?? '');
    if (!target || busy) return;
    save.mutate({
      row: tag,
      list: withRule(current, { tag, ...target }),
      done:
        target.facet === 'ignore'
          ? t('admin.radio.rules.ignoredDone', { tag })
          : t('admin.radio.rules.assignedDone', {
              tag,
              target: t('admin.radio.rules.target', {
                facet: facetLabel(t, target.facet),
                value: facetValueName(t, target.facet, target.value ?? ''),
              }),
            }),
    });
  };

  const assignOptions = [
    { value: '', label: t('admin.radio.rules.choose') },
    ...RULE_FACETS.map((facet) => facetGroup(t, facet, values[facet])),
    { value: 'ignore', label: t('admin.radio.rules.ignore') },
  ];

  const workColumns: DataColumn<WorkTag>[] = [
    {
      key: 'tag',
      header: t('admin.radio.rules.columns.tag'),
      rowHeader: true,
      cell: (entry) => <span className={styles.tag}>{entry.tag}</span>,
    },
    {
      key: 'stations',
      header: t('admin.radio.rules.columns.stations'),
      align: 'end',
      cell: (entry) => number.format(entry.stations),
    },
    {
      key: 'target',
      header: t('admin.radio.rules.columns.assign'),
      cell: (entry) => (
        <SelectField
          label={t('admin.radio.rules.chooseLabel', { tag: entry.tag })}
          hideLabel
          value={choices[entry.tag] ?? ''}
          options={assignOptions}
          onChange={(value) => setChoices((before) => ({ ...before, [entry.tag]: value }))}
        />
      ),
    },
    {
      key: 'actions',
      header: t('admin.radio.rules.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => (
        <AriaButton
          size="sm"
          isDisabled={!choices[entry.tag] || (busy && !saving(entry.tag))}
          isPending={saving(entry.tag)}
          aria-label={t('admin.radio.rules.assignLabel', { tag: entry.tag })}
          onPress={() => assign(entry.tag)}
        >
          {t('admin.radio.rules.assign')}
        </AriaButton>
      ),
    },
  ];

  /** The value a rule's select shows: the draft, else the stored target. */
  const ruleDraft = (rule: RadioTagRuleView) =>
    ruleChoices[ruleKey(rule)] ?? targetOf(rule.facet, rule.value);
  const ruleChanged = (rule: RadioTagRuleView) =>
    ruleDraft(rule) !== targetOf(rule.facet, rule.value);
  const facetName = (rule: RadioTagRuleView) =>
    rule.facet === 'ignore' ? t('admin.radio.rules.ignoreFacet') : facetLabel(t, rule.facet);

  // A choice is a draft until «Übernehmen»: the arrow keys on a closed select go through the
  // values without a save for each.
  const ruleTarget = (rule: RadioTagRuleView) => {
    if (rule.facet === 'ignore') {
      return <Text tone="secondary">{t('admin.radio.rules.ignored')}</Text>;
    }
    const facet = rule.facet;
    return (
      <SelectField
        label={t('admin.radio.rules.ruleLabel', { tag: rule.tag, facet: facetLabel(t, facet) })}
        hideLabel
        value={ruleDraft(rule)}
        options={[
          ...facetGroup(t, facet, values[facet]).options,
          { value: targetOf(facet, null), label: t('admin.radio.rules.none') },
        ]}
        onChange={(value) => setRuleChoices((before) => ({ ...before, [ruleKey(rule)]: value }))}
      />
    );
  };

  const applyRule = (rule: RadioTagRuleView) => {
    const target = parseTarget(ruleDraft(rule));
    if (!target || target.facet === 'ignore' || rule.facet === 'ignore' || busy) return;
    save.mutate({
      row: ruleKey(rule),
      list: withRule(current, { tag: rule.tag, facet: rule.facet, value: target.value }),
      done: t('admin.radio.rules.changedDone', { tag: rule.tag }),
    });
  };

  const ruleColumns: DataColumn<RadioTagRuleView>[] = [
    {
      key: 'tag',
      header: t('admin.radio.rules.columns.tag'),
      rowHeader: true,
      cell: (rule) => <span className={styles.tag}>{rule.tag}</span>,
    },
    {
      key: 'facet',
      header: t('admin.radio.rules.columns.facet'),
      cell: (rule) =>
        rule.facet === 'ignore' ? t('admin.radio.rules.ignoreFacet') : facetLabel(t, rule.facet),
    },
    { key: 'target', header: t('admin.radio.rules.columns.target'), cell: ruleTarget },
    {
      key: 'stations',
      header: t('admin.radio.rules.columns.stations'),
      align: 'end',
      cell: (rule) => number.format(rule.stations),
    },
    {
      key: 'source',
      header: t('admin.radio.rules.columns.source'),
      cell: (rule) => t(`admin.radio.rules.source.${rule.source}`),
    },
    {
      key: 'actions',
      header: t('admin.radio.rules.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (rule) => (
        <span className={styles.actions}>
          {rule.facet !== 'ignore' && (
            <AriaButton
              size="sm"
              isDisabled={!ruleChanged(rule) || (busy && !saving(ruleKey(rule)))}
              isPending={saving(ruleKey(rule))}
              aria-label={t('admin.radio.rules.applyLabel', {
                tag: rule.tag,
                facet: facetName(rule),
              })}
              onPress={() => applyRule(rule)}
            >
              {t('admin.radio.rules.apply')}
            </AriaButton>
          )}
          {rule.source === 'admin' && (
            <AriaButton
              variant="ghost"
              size="sm"
              isDisabled={busy}
              aria-label={t('admin.radio.rules.removeLabel', {
                tag: rule.tag,
                facet: facetName(rule),
              })}
              onPress={() => {
                setRemoving(rule);
                setConfirmOpen(true);
              }}
            >
              {t('admin.radio.rules.remove')}
            </AriaButton>
          )}
        </span>
      ),
    },
  ];

  const q = query.trim().toLowerCase();
  const shownRules = [...data.rules]
    .filter(
      (rule) =>
        !q ||
        rule.tag.toLowerCase().includes(q) ||
        (rule.facet !== 'ignore' &&
          rule.value !== null &&
          rule.value !== undefined &&
          facetValueName(t, rule.facet, rule.value).toLowerCase().includes(q)),
    )
    .sort((a, b) => b.stations - a.stations || a.tag.localeCompare(b.tag, language));

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      {save.error && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}

      <section className={styles.section}>
        <SectionHeader
          title={t('admin.radio.rules.coverage')}
          description={t('admin.radio.rules.coverageHelp')}
        />
        <CoverageBars
          label={t('admin.radio.rules.coverage')}
          rows={coverageRows(t, language, data.coverage)}
        />
      </section>

      <section className={styles.section} data-testid="radio-worklist">
        <SectionHeader
          title={t('admin.radio.rules.worklist')}
          description={t('admin.radio.rules.worklistHelp', {
            count: data.worklistMinStations,
          })}
        />
        {data.worklist.length === 0 ? (
          <Text tone="secondary">{t('admin.radio.rules.worklistEmpty')}</Text>
        ) : (
          <>
            <DataTable
              caption={t('admin.radio.rules.worklist')}
              columns={workColumns}
              rows={data.worklist.slice(0, workShown)}
              rowKey={(entry) => entry.tag}
            />
            {data.worklist.length > workShown && (
              <AriaButton onPress={() => setWorkShown((shown) => shown + PAGE)}>
                {t('admin.radio.rules.more', { count: data.worklist.length - workShown })}
              </AriaButton>
            )}
          </>
        )}
      </section>

      <section className={styles.section} data-testid="radio-rules">
        <SectionHeader
          title={t('admin.radio.rules.rules')}
          description={t('admin.radio.rules.rulesHelp')}
        />
        <div className={styles.filters}>
          <TextField label={t('admin.radio.rules.search')} value={query} onChange={setQuery} />
        </div>
        {shownRules.length === 0 ? (
          <Text tone="secondary">{t('admin.radio.rules.rulesEmpty')}</Text>
        ) : (
          <>
            <Text tone="secondary">
              {t('admin.radio.rules.count', { count: shownRules.length })}
            </Text>
            <DataTable
              caption={t('admin.radio.rules.rules')}
              columns={ruleColumns}
              rows={shownRules.slice(0, rulesShown)}
              rowKey={ruleKey}
            />
            {shownRules.length > rulesShown && (
              <AriaButton onPress={() => setRulesShown((shown) => shown + PAGE)}>
                {t('admin.radio.rules.more', { count: shownRules.length - rulesShown })}
              </AriaButton>
            )}
          </>
        )}
      </section>

      {removing && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={(open) => {
            setConfirmOpen(open);
            if (!open) void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
          }}
          title={t('admin.radio.rules.removeTitle', { tag: removing.tag })}
          body={
            removing.facet === 'ignore'
              ? t('admin.radio.rules.removeIgnoreBody', { tag: removing.tag })
              : t('admin.radio.rules.removeBody', { tag: removing.tag })
          }
          confirmLabel={t('admin.radio.rules.remove')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() =>
            save.mutate({
              row: ruleKey(removing),
              list: withoutRule(current, removing),
              done: t('admin.radio.rules.removedDone', { tag: removing.tag }),
            })
          }
        />
      )}
    </div>
  );
}

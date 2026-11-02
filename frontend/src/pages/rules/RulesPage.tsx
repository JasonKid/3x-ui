import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FormProvider, useForm } from 'react-hook-form';
import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Input,
  Layout,
  Modal,
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  EditOutlined,
  PartitionOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  SyncOutlined,
} from '@ant-design/icons';

import AppSidebar from '@/layouts/AppSidebar';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { keys } from '@/api/queryKeys';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useTheme } from '@/hooks/useTheme';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import './RulesPage.css';

const RULE_TYPES = [
  'DOMAIN',
  'DOMAIN-SUFFIX',
  'DOMAIN-KEYWORD',
  'IP-CIDR',
  'IP-CIDR6',
  'GEOIP',
  'USER-AGENT',
  'PROCESS-NAME',
];
const POLICIES = ['PROXY', 'DIRECT', 'REJECT'] as const;
const INTERVALS = ['ONE_HOUR', 'SIX_HOURS', 'TWELVE_HOURS', 'DAILY', 'WEEKLY'] as const;

const RuleFormSchema = z.object({
  type: z.string().min(1, 'pages.rules.required'),
  value: z.string().trim().min(1, 'pages.rules.required'),
  policy: z.string().min(1, 'pages.rules.required'),
  note: z.string().optional(),
  enabled: z.boolean(),
});

const SourceFormSchema = z.object({
  name: z.string().trim().min(1, 'pages.rules.required'),
  url: z.string().trim().url('pages.rules.urlInvalid'),
  defaultPolicy: z.string().min(1, 'pages.rules.required'),
  interval: z.string().min(1, 'pages.rules.required'),
  enabled: z.boolean(),
});

type RuleFormValues = z.infer<typeof RuleFormSchema>;
type SourceFormValues = z.infer<typeof SourceFormSchema>;

export interface RuleRow {
  id: number;
  type: string;
  value: string;
  policy: string;
  note: string;
  enabled: boolean;
  sortOrder: number;
}

export interface SourceRow {
  id: number;
  name: string;
  url: string;
  defaultPolicy: string;
  detectedFormat: string;
  interval: string;
  enabled: boolean;
  lastStatus: string;
  lastError: string;
  ruleCount: number;
  lastSyncedAt: number;
}

export interface RuleSummary {
  manualCount: number;
  externalCount: number;
  totalCount: number;
  policyCounts: Record<string, number>;
}

export interface AnalyzeResult {
  format: string;
  hasPolicy: boolean;
  missingPolicyCount: number;
  validCount: number;
  invalidCount: number;
}

interface PendingSourceDraft {
  name: string;
  url: string;
  interval: string;
  analysis: AnalyzeResult;
}

type ApiMsg<T> = { success: boolean; msg?: string; obj?: T };

async function getJson<T>(path: string): Promise<T> {
  const res = (await HttpUtil.get(path, undefined, { silent: true })) as ApiMsg<T> | undefined;
  if (!res?.success || res.obj === undefined) {
    throw new Error(res?.msg || 'request failed');
  }
  return res.obj;
}

async function postJson<T>(path: string, body?: unknown): Promise<T | undefined> {
  const res = (await HttpUtil.post(path, body, { silent: true })) as ApiMsg<T> | undefined;
  if (!res?.success) {
    throw new Error(res?.msg || 'request failed');
  }
  return res.obj;
}

function policyTagColor(policy: string): string {
  if (policy === 'DIRECT') return 'success';
  if (policy === 'REJECT') return 'error';
  return 'geekblue';
}

const ruleDefaults: RuleFormValues = {
  type: 'DOMAIN-SUFFIX',
  value: '',
  policy: 'PROXY',
  note: '',
  enabled: true,
};

const sourceDefaults: SourceFormValues = {
  name: '',
  url: '',
  defaultPolicy: 'PROXY',
  interval: 'DAILY',
  enabled: true,
};

export default function RulesPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, contextHolder] = message.useMessage();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const rulesQuery = useQuery({
    queryKey: keys.rules.list(),
    queryFn: () => getJson<RuleRow[]>('/panel/api/rules/list'),
    retry: false,
  });
  const sourcesQuery = useQuery({
    queryKey: keys.rules.sources(),
    queryFn: () => getJson<SourceRow[]>('/panel/api/rules/sources'),
    retry: false,
  });
  const summaryQuery = useQuery({
    queryKey: keys.rules.summary(),
    queryFn: () => getJson<RuleSummary>('/panel/api/rules/summary'),
    retry: false,
  });

  const rules = rulesQuery.data ?? [];
  const sources = sourcesQuery.data ?? [];
  const summary = summaryQuery.data ?? {
    manualCount: rules.length,
    externalCount: 0,
    totalCount: rules.length,
    policyCounts: { PROXY: 0, DIRECT: 0, REJECT: 0 },
  };
  const fetched = rulesQuery.isFetched && sourcesQuery.isFetched;
  const loading = rulesQuery.isFetching || sourcesQuery.isFetching || summaryQuery.isFetching;

  const [quickType, setQuickType] = useState('DOMAIN-SUFFIX');
  const [quickValue, setQuickValue] = useState('');
  const [quickPolicy, setQuickPolicy] = useState('PROXY');
  const [quickNote, setQuickNote] = useState('');
  const [addingRule, setAddingRule] = useState(false);

  const [quickSourceName, setQuickSourceName] = useState('');
  const [quickSourceUrl, setQuickSourceUrl] = useState('');
  const [quickSourceInterval, setQuickSourceInterval] = useState('DAILY');
  const [addingSource, setAddingSource] = useState(false);

  const [pendingSource, setPendingSource] = useState<PendingSourceDraft | null>(null);
  const [pendingPolicy, setPendingPolicy] = useState('PROXY');
  const [confirmingSource, setConfirmingSource] = useState(false);

  const [ruleOpen, setRuleOpen] = useState(false);
  const [sourceOpen, setSourceOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<RuleRow | null>(null);
  const [editingSource, setEditingSource] = useState<SourceRow | null>(null);
  const [syncingId, setSyncingId] = useState<number | null>(null);

  const ruleMethods = useForm<RuleFormValues>({ defaultValues: ruleDefaults });
  const sourceMethods = useForm<SourceFormValues>({ defaultValues: sourceDefaults });

  const refreshAll = async () => {
    await Promise.all([rulesQuery.refetch(), sourcesQuery.refetch(), summaryQuery.refetch()]);
  };

  const intervalLabel = (interval: string) => {
    switch (interval) {
      case 'ONE_HOUR':
        return t('pages.rules.intervalOneHour');
      case 'SIX_HOURS':
        return t('pages.rules.intervalSixHours');
      case 'TWELVE_HOURS':
        return t('pages.rules.intervalTwelveHours');
      case 'WEEKLY':
        return t('pages.rules.intervalWeekly');
      default:
        return t('pages.rules.intervalDaily');
    }
  };

  const handleQuickAddRule = async () => {
    const trimmed = quickValue.trim();
    if (!trimmed) {
      messageApi.warning(t('pages.rules.required'));
      return;
    }
    setAddingRule(true);
    try {
      await postJson('/panel/api/rules/add', {
        type: quickType,
        value: trimmed,
        policy: quickPolicy,
        note: quickNote.trim(),
        enabled: true,
      });
      setQuickValue('');
      setQuickNote('');
      await refreshAll();
      messageApi.success(t('pages.rules.ruleAdded'));
    } catch (err) {
      messageApi.error((err as Error).message);
    } finally {
      setAddingRule(false);
    }
  };

  const handleQuickAddSource = async () => {
    const name = quickSourceName.trim();
    const url = quickSourceUrl.trim();
    if (!name || !url) {
      messageApi.warning(t('pages.rules.required'));
      return;
    }
    setAddingSource(true);
    try {
      const analysis = await postJson<AnalyzeResult>('/panel/api/rules/source/analyze', { url });
      if (analysis && analysis.missingPolicyCount > 0) {
        setPendingSource({
          name,
          url,
          interval: quickSourceInterval,
          analysis,
        });
        setPendingPolicy('PROXY');
        return;
      }
      await postJson('/panel/api/rules/source/add', {
        name,
        url,
        interval: quickSourceInterval,
        defaultPolicy: 'PROXY',
        enabled: true,
      });
      setQuickSourceName('');
      setQuickSourceUrl('');
      await refreshAll();
      messageApi.success(t('pages.rules.sourceAdded'));
    } catch (err) {
      messageApi.error((err as Error).message);
    } finally {
      setAddingSource(false);
    }
  };

  const confirmPendingSource = async () => {
    if (!pendingSource) return;
    setConfirmingSource(true);
    try {
      await postJson('/panel/api/rules/source/add', {
        name: pendingSource.name,
        url: pendingSource.url,
        interval: pendingSource.interval,
        defaultPolicy: pendingPolicy,
        enabled: true,
      });
      setPendingSource(null);
      setQuickSourceName('');
      setQuickSourceUrl('');
      await refreshAll();
      messageApi.success(t('pages.rules.sourceAdded'));
    } catch (err) {
      messageApi.error((err as Error).message);
    } finally {
      setConfirmingSource(false);
    }
  };

  const openEditRule = (row: RuleRow) => {
    setEditingRule(row);
    ruleMethods.reset({
      type: row.type,
      value: row.value,
      policy: row.policy,
      note: row.note,
      enabled: row.enabled,
    });
    setRuleOpen(true);
  };

  const openEditSource = (row: SourceRow) => {
    setEditingSource(row);
    sourceMethods.reset({
      name: row.name,
      url: row.url,
      defaultPolicy: row.defaultPolicy,
      interval: row.interval,
      enabled: row.enabled,
    });
    setSourceOpen(true);
  };

  const submitRule = ruleMethods.handleSubmit(async (values) => {
    try {
      if (editingRule) {
        await postJson('/panel/api/rules/update', { ...values, id: editingRule.id });
      } else {
        await postJson('/panel/api/rules/add', values);
      }
      setRuleOpen(false);
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  });

  const submitSource = sourceMethods.handleSubmit(async (values) => {
    try {
      if (editingSource) {
        await postJson('/panel/api/rules/source/update', { ...values, id: editingSource.id });
      } else {
        await postJson('/panel/api/rules/source/add', values);
      }
      setSourceOpen(false);
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  });

  const toggleRuleEnabled = async (row: RuleRow, enabled: boolean) => {
    try {
      await postJson('/panel/api/rules/update', { ...row, enabled });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const toggleSourceEnabled = async (row: SourceRow, enabled: boolean) => {
    try {
      await postJson('/panel/api/rules/source/update', { ...row, enabled });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const moveRule = async (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= rules.length) return;
    const next = [...rules];
    const [moved] = next.splice(index, 1);
    next.splice(target, 0, moved);
    try {
      await postJson('/panel/api/rules/reorder', { ids: next.map((r) => r.id) });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const moveSource = async (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= sources.length) return;
    const next = [...sources];
    const [moved] = next.splice(index, 1);
    next.splice(target, 0, moved);
    try {
      await postJson('/panel/api/rules/source/reorder', { ids: next.map((s) => s.id) });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const removeRule = async (id: number) => {
    try {
      await postJson('/panel/api/rules/delete', { id });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const removeSource = async (id: number) => {
    try {
      await postJson('/panel/api/rules/source/delete', { id });
      await refreshAll();
    } catch (err) {
      messageApi.error((err as Error).message);
    }
  };

  const syncSource = async (id: number) => {
    setSyncingId(id);
    try {
      await postJson('/panel/api/rules/source/sync', { id });
      await refreshAll();
      messageApi.success(t('pages.rules.synced'));
    } catch (err) {
      messageApi.error((err as Error).message);
    } finally {
      setSyncingId(null);
    }
  };

  const ruleColumns: TableColumnsType<RuleRow> = [
    {
      title: '#',
      width: 90,
      render: (_v: unknown, _row: RuleRow, index: number) => (
        <Space size={2}>
          <Button
            size="small"
            type="text"
            disabled={index === 0}
            icon={<ArrowUpOutlined aria-hidden />}
            onClick={() => void moveRule(index, -1)}
          />
          <Button
            size="small"
            type="text"
            disabled={index === rules.length - 1}
            icon={<ArrowDownOutlined aria-hidden />}
            onClick={() => void moveRule(index, 1)}
          />
        </Space>
      ),
    },
    {
      title: t('pages.rules.type'),
      dataIndex: 'type',
      width: 150,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('pages.rules.value'),
      dataIndex: 'value',
      render: (v: string) => <span className="rules-mono">{v}</span>,
    },
    {
      title: t('pages.rules.policy'),
      dataIndex: 'policy',
      width: 110,
      render: (v: string) => <Tag color={policyTagColor(v)}>{v}</Tag>,
    },
    {
      title: t('pages.rules.note'),
      dataIndex: 'note',
      width: 180,
    },
    {
      title: t('pages.rules.enabled'),
      dataIndex: 'enabled',
      width: 90,
      render: (v: boolean, row: RuleRow) => (
        <Switch
          size="small"
          aria-label={t('pages.rules.enabled')}
          checked={v}
          onChange={(next) => void toggleRuleEnabled(row, next)}
        />
      ),
    },
    {
      title: t('pages.rules.actions'),
      width: 110,
      render: (_: unknown, row: RuleRow) => (
        <Space size={4}>
          <Button
            size="small"
            icon={<EditOutlined aria-hidden />}
            onClick={() => openEditRule(row)}
          />
          <Popconfirm
            title={t('pages.rules.deleteConfirm')}
            onConfirm={() => void removeRule(row.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined aria-hidden />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const sourceColumns: TableColumnsType<SourceRow> = [
    {
      title: '#',
      width: 90,
      render: (_v: unknown, _row: SourceRow, index: number) => (
        <Space size={2}>
          <Button
            size="small"
            type="text"
            disabled={index === 0}
            icon={<ArrowUpOutlined aria-hidden />}
            onClick={() => void moveSource(index, -1)}
          />
          <Button
            size="small"
            type="text"
            disabled={index === sources.length - 1}
            icon={<ArrowDownOutlined aria-hidden />}
            onClick={() => void moveSource(index, 1)}
          />
        </Space>
      ),
    },
    {
      title: t('pages.rules.name'),
      dataIndex: 'name',
      width: 160,
    },
    {
      title: t('pages.rules.url'),
      dataIndex: 'url',
      render: (v: string) => <span className="rules-mono">{v}</span>,
    },
    {
      title: t('pages.rules.policy'),
      dataIndex: 'defaultPolicy',
      width: 100,
      render: (v: string) => <Tag color={policyTagColor(v)}>{v}</Tag>,
    },
    {
      title: t('pages.rules.interval'),
      dataIndex: 'interval',
      width: 130,
      render: (v: string) => <Tag>{intervalLabel(v)}</Tag>,
    },
    {
      title: t('pages.rules.ruleCount'),
      dataIndex: 'ruleCount',
      width: 100,
      render: (v: number) => <Tag>{v}</Tag>,
    },
    {
      title: t('pages.rules.status'),
      dataIndex: 'lastStatus',
      width: 110,
      render: (v: string, row: SourceRow) => {
        if (v === 'SUCCESS' || v === 'OK') {
          return <Tag color="success">{t('pages.rules.statusSuccess')}</Tag>;
        }
        if (v === 'FAILED') {
          return (
            <Tooltip title={row.lastError}>
              <Tag color="error">{t('pages.rules.statusFailed')}</Tag>
            </Tooltip>
          );
        }
        return <Tag>{t('pages.rules.statusNever')}</Tag>;
      },
    },
    {
      title: t('pages.rules.enabled'),
      dataIndex: 'enabled',
      width: 90,
      render: (v: boolean, row: SourceRow) => (
        <Switch
          size="small"
          aria-label={t('pages.rules.enabled')}
          checked={v}
          onChange={(next) => void toggleSourceEnabled(row, next)}
        />
      ),
    },
    {
      title: t('pages.rules.actions'),
      width: 140,
      render: (_: unknown, row: SourceRow) => (
        <Space size={4}>
          <Button
            size="small"
            loading={syncingId === row.id}
            icon={<SyncOutlined aria-hidden />}
            onClick={() => void syncSource(row.id)}
          />
          <Button
            size="small"
            icon={<EditOutlined aria-hidden />}
            onClick={() => openEditSource(row)}
          />
          <Popconfirm
            title={t('pages.rules.deleteConfirm')}
            onConfirm={() => void removeSource(row.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined aria-hidden />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const pageClass = useMemo(() => {
    const classes = ['rules-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {contextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                <Col span={24}>
                  <Card size="small" hoverable className="summary-card">
                    <Row gutter={[16, isMobile ? 16 : 12]} align="middle">
                      <Col xs={12} sm={8} md={6}>
                        <Statistic
                          title={t('pages.rules.manualCount')}
                          value={summary.manualCount}
                          prefix={<PartitionOutlined aria-hidden />}
                        />
                      </Col>
                      <Col xs={12} sm={8} md={6}>
                        <Statistic
                          title={t('pages.rules.externalCount')}
                          value={summary.externalCount}
                          prefix={<DatabaseOutlined aria-hidden />}
                        />
                      </Col>
                      <Col xs={24} sm={8} md={12}>
                        <div className="rules-policy-dist">
                          <div className="rules-policy-row">
                            <span>
                              <SafetyCertificateOutlined aria-hidden />{' '}
                              {t('pages.rules.policyDistribution')}
                            </span>
                          </div>
                          {POLICIES.map((pol) => {
                            const count = summary.policyCounts?.[pol] ?? 0;
                            const pct =
                              summary.totalCount > 0
                                ? Math.round((count / summary.totalCount) * 100)
                                : 0;
                            return (
                              <div key={pol} className="rules-policy-row">
                                <Space size={6}>
                                  <Tag color={policyTagColor(pol)}>{pol}</Tag>
                                  <span>{count}</span>
                                </Space>
                                <Progress
                                  percent={pct}
                                  size="small"
                                  style={{ maxWidth: 220, margin: 0 }}
                                />
                              </div>
                            );
                          })}
                        </div>
                      </Col>
                    </Row>
                  </Card>
                </Col>

                <Col span={24}>
                  <Card size="small" hoverable title={t('pages.rules.tabManual')}>
                    <div className="rules-inline-bar">
                      <Select
                        aria-label={t('pages.rules.type')}
                        value={quickType}
                        onChange={setQuickType}
                        options={RULE_TYPES.map((v) => ({ value: v, label: v }))}
                      />
                      <Input
                        aria-label={t('pages.rules.value')}
                        value={quickValue}
                        placeholder="example.com"
                        onChange={(e) => setQuickValue(e.target.value)}
                        onPressEnter={() => void handleQuickAddRule()}
                      />
                      <Select
                        aria-label={t('pages.rules.policy')}
                        value={quickPolicy}
                        onChange={setQuickPolicy}
                        options={POLICIES.map((v) => ({ value: v, label: v }))}
                      />
                      <Input
                        aria-label={t('pages.rules.note')}
                        value={quickNote}
                        placeholder={t('pages.rules.note')}
                        onChange={(e) => setQuickNote(e.target.value)}
                        onPressEnter={() => void handleQuickAddRule()}
                      />
                      <Button
                        type="primary"
                        loading={addingRule}
                        icon={<PlusOutlined aria-hidden />}
                        onClick={() => void handleQuickAddRule()}
                      >
                        {t('pages.rules.add')}
                      </Button>
                    </div>
                    <Table<RuleRow>
                      rowKey="id"
                      size="small"
                      loading={loading}
                      columns={ruleColumns}
                      dataSource={rules}
                      pagination={{ pageSize: 20, hideOnSinglePage: true }}
                    />
                  </Card>
                </Col>

                <Col span={24}>
                  <Card size="small" hoverable title={t('pages.rules.tabSources')}>
                    <div className="rules-source-bar">
                      <Input
                        aria-label={t('pages.rules.name')}
                        value={quickSourceName}
                        placeholder={t('pages.rules.name')}
                        onChange={(e) => setQuickSourceName(e.target.value)}
                      />
                      <Input
                        aria-label={t('pages.rules.url')}
                        value={quickSourceUrl}
                        placeholder="https://example.com/rules.list"
                        onChange={(e) => setQuickSourceUrl(e.target.value)}
                        onPressEnter={() => void handleQuickAddSource()}
                      />
                      <Select
                        aria-label={t('pages.rules.interval')}
                        value={quickSourceInterval}
                        onChange={setQuickSourceInterval}
                        options={INTERVALS.map((v) => ({ value: v, label: intervalLabel(v) }))}
                      />
                      <Button
                        type="primary"
                        loading={addingSource}
                        icon={<PlusOutlined aria-hidden />}
                        onClick={() => void handleQuickAddSource()}
                      >
                        {t('pages.rules.add')}
                      </Button>
                    </div>
                    <Table<SourceRow>
                      rowKey="id"
                      size="small"
                      loading={loading}
                      columns={sourceColumns}
                      dataSource={sources}
                      pagination={{ pageSize: 20, hideOnSinglePage: true }}
                    />
                  </Card>
                </Col>
              </Row>
            </Spin>
          </Layout.Content>
        </Layout>

        <Modal
          open={pendingSource !== null}
          title={t('pages.rules.chooseDefaultPolicy')}
          onCancel={() => setPendingSource(null)}
          onOk={() => void confirmPendingSource()}
          confirmLoading={confirmingSource}
          destroyOnHidden
        >
          {pendingSource && (
            <Space direction="vertical" style={{ width: '100%' }} size={12}>
              <div>
                {t('pages.rules.missingPolicyHint', {
                  count: pendingSource.analysis.missingPolicyCount,
                })}
              </div>
              <div className="rules-mono">
                {pendingSource.analysis.format} · {pendingSource.analysis.validCount} /{' '}
                {pendingSource.analysis.invalidCount}
              </div>
              <Select
                aria-label={t('pages.rules.policy')}
                style={{ width: '100%' }}
                value={pendingPolicy}
                onChange={setPendingPolicy}
                options={POLICIES.map((v) => ({ value: v, label: v }))}
              />
            </Space>
          )}
        </Modal>

        <Modal
          open={ruleOpen}
          title={editingRule ? t('pages.rules.editRule') : t('pages.rules.addRule')}
          onCancel={() => setRuleOpen(false)}
          onOk={() => void submitRule()}
          destroyOnHidden
        >
          <FormProvider {...ruleMethods}>
            <FormField
              label={t('pages.rules.type')}
              name="type"
              rules={{ validate: rhfZodValidate(RuleFormSchema.shape.type) }}
            >
              <Select options={RULE_TYPES.map((v) => ({ value: v, label: v }))} />
            </FormField>
            <FormField
              label={t('pages.rules.value')}
              name="value"
              rules={{ validate: rhfZodValidate(RuleFormSchema.shape.value) }}
            >
              <Input placeholder="example.com" />
            </FormField>
            <FormField
              label={t('pages.rules.policy')}
              name="policy"
              rules={{ validate: rhfZodValidate(RuleFormSchema.shape.policy) }}
            >
              <Select options={POLICIES.map((v) => ({ value: v, label: v }))} />
            </FormField>
            <FormField label={t('pages.rules.note')} name="note">
              <Input />
            </FormField>
            <FormField label={t('pages.rules.enabled')} name="enabled" valueProp="checked">
              <Switch />
            </FormField>
          </FormProvider>
        </Modal>

        <Modal
          open={sourceOpen}
          title={editingSource ? t('pages.rules.editSource') : t('pages.rules.addSource')}
          onCancel={() => setSourceOpen(false)}
          onOk={() => void submitSource()}
          destroyOnHidden
        >
          <FormProvider {...sourceMethods}>
            <FormField
              label={t('pages.rules.name')}
              name="name"
              rules={{ validate: rhfZodValidate(SourceFormSchema.shape.name) }}
            >
              <Input />
            </FormField>
            <FormField
              label={t('pages.rules.url')}
              name="url"
              rules={{ validate: rhfZodValidate(SourceFormSchema.shape.url) }}
            >
              <Input placeholder="https://example.com/rules.list" />
            </FormField>
            <FormField
              label={t('pages.rules.policy')}
              name="defaultPolicy"
              rules={{ validate: rhfZodValidate(SourceFormSchema.shape.defaultPolicy) }}
            >
              <Select options={POLICIES.map((v) => ({ value: v, label: v }))} />
            </FormField>
            <FormField
              label={t('pages.rules.interval')}
              name="interval"
              rules={{ validate: rhfZodValidate(SourceFormSchema.shape.interval) }}
            >
              <Select options={INTERVALS.map((v) => ({ value: v, label: intervalLabel(v) }))} />
            </FormField>
            <FormField label={t('pages.rules.enabled')} name="enabled" valueProp="checked">
              <Switch />
            </FormField>
          </FormProvider>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}

import { writable, derived } from 'svelte/store';

export type Language = 'en' | 'zh';

const STORAGE_KEY = 'doo_language';

function getInitialLanguage(): Language {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'zh' || saved === 'en') {
      return saved;
    }
  } catch (e) {
    // Ignore localStorage error
  }
  return 'en'; // Default English
}

export const currentLanguage = writable<Language>(getInitialLanguage());

export function setLanguage(lang: Language) {
  currentLanguage.set(lang);
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch (e) {
    // Ignore
  }
}

export const translations = {
  en: {
    // Nav
    appName: 'Doo',
    appTagline: 'LLM API Benchmark Pilot Study',
    navCompare: 'Compare',
    navConfig: 'Config',
    navHistory: 'History',

    // Compare Page
    selectModels: 'Select Models',
    noModelsConfigured: 'No models configured yet.',
    goToConfig: 'Go to Configuration',
    systemPrompt: 'System Prompt',
    systemPromptPlaceholder: 'You are a helpful and concise assistant...',
    collapse: 'Collapse',
    expand: 'Expand',
    messages: 'Messages',
    addMessage: 'Add Message',
    remove: 'Remove',
    moveUp: 'Move Up',
    moveDown: 'Move Down',
    userRole: 'User',
    assistantRole: 'Assistant',
    messagePlaceholder: 'Enter message content...',
    previewBody: 'Preview Request Body',
    send: 'Send',
    stopAll: 'Stop All',
    stop: 'Stop',
    timeoutSec: 'Timeout (s)',
    notePlaceholder: 'Optional note for this run...',
    noModelsSelected: 'Please select at least one model to run.',
    selectUpTo8: 'You can select up to 8 models at once.',
    selectUpToN: 'You can select up to {n} models at once.',
    maxTargetsTooltip: 'Up to {max} models can be compared concurrently (configurable via startup flag -max-targets)',
    unlimitedTargetsTooltip: 'No limit on concurrent models (configurable via startup flag -max-targets)',

    // Card Tabs & Badges
    tabOutput: 'Output',
    tabReasoning: 'Reasoning',
    tabRequest: 'Request',
    tabResponse: 'Response',
    markdownRender: 'Render Markdown',
    plainText: 'Plain Text',
    statusRunning: 'Running',
    statusDone: 'Done',
    statusError: 'Error',
    statusCanceled: 'Canceled',
    copyOutput: 'Copy Output',
    copied: 'Copied!',
    exportCurl: 'Export curl',
    exportPython: 'Export Python',
    close: 'Close',

    // Metrics
    metricTTFT: 'First Token',
    metricFirstContent: 'First Content',
    metricTotal: 'Total Time',
    metricTokens: 'Tokens (In/Out/Think)',
    metricSpeed: 'Tokens/s',
    metricFinish: 'Finish Reason',
    metricModel: 'Resp Model',
    loadingRaw: 'Loading raw payload...',

    // Config Page
    providerRoleTitle: '1. Service Providers (Gateways & Credentials)',
    modelRoleTitle: '2. Models & Presets (Models & Parameters)',
    providersTitle: 'Service Providers',
    addProvider: 'Add Provider',
    editProvider: 'Edit Provider',
    providerName: 'Provider Name',
    protocol: 'Protocol',
    baseUrl: 'Base URL',
    apiKey: 'API Key',
    apiKeyPlaceholder: 'sk-... (encrypted & stored locally)',
    leaveEmptyToKeep: 'leave empty to keep current key',
    headersJson: 'Custom Headers (JSON)',
    streamUsageDesc: 'Send stream_options.include_usage (uncheck if provider does not support it)',
    testConnection: 'Test Connection',
    testing: 'Testing...',
    testSuccess: 'Connected',
    testFailed: 'Failed',
    providerTemplates: 'Quick Templates',
    officialDocs: 'Official Model Docs',
    viewOfficialDocs: 'View Official Docs',
    officialDocUrl: 'Official Docs',
    checkDocsForModels: 'Refer to official documentation for supported model names',
    showApiKey: 'Show API Key',
    hideApiKey: 'Hide API Key',
    noModelsUnderProvider: 'No model presets configured under this provider yet.',
    fetchModels: 'Fetch Models (/v1/models)',
    fetchingModels: 'Fetching...',
    discoveredModels: 'Discovered Models from Server',
    addSelectedModels: 'Add Selected Models',
    selectAll: 'Select All',
    deselectAll: 'Deselect All',
    alreadyAdded: 'Already Added',
    noModelsFound: 'No models found from /v1/models. Ensure the local server is running.',
    pickFromDiscovered: 'Or pick from discovered models',

    targetsTitle: 'Models & Presets',
    modelsUnderProvider: 'Models under this Provider',
    selectProviderFirst: 'Select or add a provider on the left to manage its models.',
    addTarget: 'Add Model',
    editTarget: 'Edit Model',
    targetLabel: 'Model Preset Label',
    providerLabel: 'Provider',
    modelLabel: 'Model',
    modelName: 'Model Name',
    coreParams: 'Core Parameters (directly editable; unchecked fields use provider defaults)',
    tempParam: 'Temperature',
    topPParam: 'Top P',
    maxTokensParam: 'Max Tokens',
    streamParam: 'Stream (Streaming)',
    customizeAndAdd: 'Customize & Add',
    stopParam: 'Stop Sequences (comma separated)',
    seedParam: 'Seed (random seed)',
    maxTokensField: 'Max Tokens Field Name',
    extraJson: 'Extra Parameters (JSON, deep merged)',
    extraJsonHint: 'Keys set to null will be removed from the request.',
    duplicate: 'Duplicate',
    delete: 'Delete',
    save: 'Save',
    cancel: 'Cancel',
    confirmDeleteProvider: 'Are you sure you want to delete this provider? All its models will also be deleted.',
    confirmDeleteTarget: 'Are you sure you want to delete this model preset?',

    // History Page
    historyTitle: 'Run History',
    noHistory: 'No runs recorded yet.',
    loadToCompare: 'Load into Compare',
    deleteRun: 'Delete Run',
    confirmDeleteRun: 'Are you sure you want to delete this run record?',
    inputSnippet: 'Input Prompt',
    resultsSummary: 'Results',
    viewDetails: 'View Details',
    hideDetails: 'Hide Details',
    targetDeletedNotice: 'This model preset has been deleted from settings; showing historical snapshot.',

    // Common & Modal
    previewModalTitle: 'Request Body Preview',
    exportModalTitle: 'Export Snippet',
    copyCode: 'Copy Code',
    loading: 'Loading...',
    errorOccurred: 'An error occurred',
  },
  zh: {
    // 导航
    appName: 'Doo',
    appTagline: 'LLM API Benchmark 前期评测工具',
    navCompare: '对比',
    navConfig: '配置',
    navHistory: '历史',

    // 对比页
    selectModels: '选择模型',
    noModelsConfigured: '暂未配置任何模型。',
    goToConfig: '前往配置',
    systemPrompt: 'System Prompt',
    systemPromptPlaceholder: 'You are a helpful and concise assistant...',
    collapse: '折叠',
    expand: '展开',
    messages: '多轮消息',
    addMessage: '添加消息',
    remove: '删除',
    moveUp: '上移',
    moveDown: '下移',
    userRole: '用户',
    assistantRole: '模型',
    messagePlaceholder: '输入消息内容...',
    previewBody: '预览请求体',
    send: '发送',
    stopAll: '全部停止',
    stop: '停止',
    timeoutSec: '超时(秒)',
    notePlaceholder: '本轮测试备注(可选)...',
    noModelsSelected: '请至少勾选一个模型。',
    selectUpTo8: '最多可同时选择 8 个模型。',
    selectUpToN: '最多可同时选择 {n} 个模型。',
    maxTargetsTooltip: '最多支持并发对比 {max} 个模型（可通过启动参数 -max-targets 调整）',
    unlimitedTargetsTooltip: '未限制并发对比模型数（可通过启动参数 -max-targets 调整）',

    // 卡片 Tabs & 徽标
    tabOutput: '输出',
    tabReasoning: '推理',
    tabRequest: '请求',
    tabResponse: '响应',
    markdownRender: 'Markdown 渲染',
    plainText: '纯文本',
    statusRunning: '运行中',
    statusDone: '已完成',
    statusError: '出错',
    statusCanceled: '已取消',
    copyOutput: '复制输出',
    copied: '已复制！',
    exportCurl: '导出 curl',
    exportPython: '导出 Python',
    close: '关闭',

    // 指标
    metricTTFT: '首 Token',
    metricFirstContent: '首正文',
    metricTotal: '总耗时',
    metricTokens: 'Tokens (输入/输出/推理)',
    metricSpeed: 'Tokens/s',
    metricFinish: '结束原因',
    metricModel: '返回模型',
    loadingRaw: '正在加载原始报文...',

    // 配置页
    providerRoleTitle: '1. 服务商管理 (API 网关与凭证)',
    modelRoleTitle: '2. 模型预设 (参数与模型名称)',
    providersTitle: '服务商 (Provider)',
    addProvider: '添加服务商',
    editProvider: '编辑服务商',
    providerName: '服务商名称',
    protocol: '协议类型',
    baseUrl: 'Base URL',
    apiKey: 'API Key 密钥',
    apiKeyPlaceholder: 'sk-... (本地 AES-256-GCM 加密安全存储)',
    leaveEmptyToKeep: '留空表示保持当前密钥不变',
    headersJson: '额外请求头 (JSON)',
    streamUsageDesc: '发送 stream_options.include_usage (兼容服务若不支持请取消勾选)',
    testConnection: '测试连接',
    testing: '测试中...',
    testSuccess: '连接成功',
    testFailed: '连接失败',
    providerTemplates: '常用模板',
    officialDocs: '官方模型文档',
    viewOfficialDocs: '查看官方文档',
    officialDocUrl: '官方文档',
    checkDocsForModels: '建议前往官方文档查看最新支持的模型名称',
    showApiKey: '显示 API Key',
    hideApiKey: '隐藏 API Key',
    noModelsUnderProvider: '该服务商下暂未配置模型预设。',
    fetchModels: '获取可选模型 (/v1/models)',
    fetchingModels: '正在获取...',
    discoveredModels: '从服务获取到的可选模型',
    addSelectedModels: '一键添加勾选的模型',
    selectAll: '全选',
    deselectAll: '取消全选',
    alreadyAdded: '已添加',
    noModelsFound: '未能从 /v1/models 获取到模型，请确认本地服务已启动并已下载模型。',
    pickFromDiscovered: '或从已发现的模型中选择',

    targetsTitle: '模型与预设参数',
    modelsUnderProvider: '该服务商下的模型预设',
    selectProviderFirst: '在左侧选择或新建服务商，即可管理其下的模型配置。',
    addTarget: '添加模型预设',
    editTarget: '编辑模型预设',
    targetLabel: '模型配置名称',
    providerLabel: '服务商',
    modelLabel: '模型',
    modelName: '模型名称 (model_name)',
    coreParams: '核心参数 (支持直接修改数值；未勾选则使用服务商默认)',
    tempParam: 'Temperature 温度',
    topPParam: 'Top P',
    maxTokensParam: 'Max Tokens 最大输出',
    streamParam: 'Stream 流式传输',
    customizeAndAdd: '自定义并添加',
    stopParam: 'Stop 停止词 (英文逗号分隔)',
    seedParam: 'Seed 随机种子',
    maxTokensField: '最大 Token 字段名',
    extraJson: '厂商特有参数 (JSON, 深度合并)',
    extraJsonHint: '值为 null 的键会在发送时被删除。',
    duplicate: '复制',
    delete: '删除',
    save: '保存',
    cancel: '取消',
    confirmDeleteProvider: '确定要删除此服务商吗？关联的所有模型也将被同时删除。',
    confirmDeleteTarget: '确定要删除此模型预设吗？',

    // 历史页
    historyTitle: '测试历史',
    noHistory: '暂无测试历史记录。',
    loadToCompare: '载入到对比页',
    deleteRun: '删除记录',
    confirmDeleteRun: '确定要删除此条测试历史吗？',
    inputSnippet: '输入内容',
    resultsSummary: '结果概要',
    viewDetails: '查看详情',
    hideDetails: '收起详情',
    targetDeletedNotice: '此模型预设已在设置中被删除，当前展示历史快照。',

    // 通用 & 弹窗
    previewModalTitle: '请求体预览',
    exportModalTitle: '导出代码片段',
    copyCode: '复制代码',
    loading: '加载中...',
    errorOccurred: '发生错误',
  },
};

export const t = derived(currentLanguage, ($lang) => {
  return (key: keyof typeof translations['en']): string => {
    return translations[$lang][key] || translations['en'][key] || key;
  };
});

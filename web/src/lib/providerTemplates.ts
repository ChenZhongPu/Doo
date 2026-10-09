export interface DocItem {
  title: string;
  url: string;
}

export interface ProviderTemplate {
  id: string;
  name: string;
  brand: string;
  protocol: 'openai_chat' | 'anthropic';
  baseUrl: string;
  defaultEnv?: string;
  docUrl?: string; // Official provider models documentation URL
  extraDocs?: DocItem[]; // Additional documentation (e.g. llama.cpp for LM Studio)
  headers?: Record<string, string>;
}

export const PROVIDER_TEMPLATES: ProviderTemplate[] = [
  {
    id: 'deepseek',
    name: 'DeepSeek',
    brand: 'deepseek',
    protocol: 'openai_chat',
    baseUrl: 'https://api.deepseek.com',
    defaultEnv: 'DEEPSEEK_API_KEY',
    docUrl: 'https://api-docs.deepseek.com/',
  },
  {
    id: 'qwen',
    name: 'Qwen (通义千问)',
    brand: 'qwen',
    protocol: 'openai_chat',
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    defaultEnv: 'DASHSCOPE_API_KEY',
    docUrl: 'https://help.aliyun.com/zh/model-studio/getting-started/models',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    brand: 'openrouter',
    protocol: 'openai_chat',
    baseUrl: 'https://openrouter.ai/api/v1',
    defaultEnv: 'OPENROUTER_API_KEY',
    docUrl: 'https://openrouter.ai/models',
    headers: {
      'HTTP-Referer': 'http://localhost:8787',
      'X-Title': 'Doo',
    },
  },
  {
    id: 'glm',
    name: 'GLM (智谱 AI)',
    brand: 'glm',
    protocol: 'openai_chat',
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4',
    defaultEnv: 'ZHIPU_API_KEY',
    docUrl: 'https://open.bigmodel.cn/dev/api',
  },
  {
    id: 'openai',
    name: 'OpenAI',
    brand: 'openai',
    protocol: 'openai_chat',
    baseUrl: 'https://api.openai.com/v1',
    defaultEnv: 'OPENAI_API_KEY',
    docUrl: 'https://platform.openai.com/docs/models',
  },
  {
    id: 'anthropic',
    name: 'Anthropic',
    brand: 'anthropic',
    protocol: 'anthropic',
    baseUrl: 'https://api.anthropic.com/v1',
    defaultEnv: 'ANTHROPIC_API_KEY',
    docUrl: 'https://docs.anthropic.com/en/docs/about-claude/models',
  },
  {
    id: 'siliconflow',
    name: 'SiliconFlow (硅基流动)',
    brand: 'siliconflow',
    protocol: 'openai_chat',
    baseUrl: 'https://api.siliconflow.cn/v1',
    defaultEnv: 'SILICONFLOW_API_KEY',
    docUrl: 'https://docs.siliconflow.cn/docs/model-names',
  },
  {
    id: 'groq',
    name: 'Groq',
    brand: 'groq',
    protocol: 'openai_chat',
    baseUrl: 'https://api.groq.com/openai/v1',
    defaultEnv: 'GROQ_API_KEY',
    docUrl: 'https://console.groq.com/docs/models',
  },
  {
    id: 'moonshot',
    name: 'Moonshot (Kimi)',
    brand: 'moonshot',
    protocol: 'openai_chat',
    baseUrl: 'https://api.moonshot.cn/v1',
    defaultEnv: 'MOONSHOT_API_KEY',
    docUrl: 'https://platform.moonshot.cn/docs',
  },
  {
    id: 'lmstudio',
    name: 'LM Studio',
    brand: 'lmstudio',
    protocol: 'openai_chat',
    baseUrl: 'http://localhost:1234/v1',
    defaultEnv: 'LMSTUDIO_API_KEY',
    docUrl: 'https://lmstudio.ai/docs/local-server',
    extraDocs: [
      {
        title: 'llama.cpp Server Docs',
        url: 'https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md',
      },
    ],
  },
  {
    id: 'ollama',
    name: 'Ollama',
    brand: 'ollama',
    protocol: 'openai_chat',
    baseUrl: 'http://localhost:11434/v1',
    defaultEnv: 'OLLAMA_API_KEY',
    docUrl: 'https://ollama.com/library',
  },
  {
    id: 'custom',
    name: 'Custom Provider',
    brand: 'custom',
    protocol: 'openai_chat',
    baseUrl: '',
    defaultEnv: 'API_KEY',
  },
];

export function getProviderDocs(provider?: { name?: string; base_url?: string } | null): DocItem[] {
  if (!provider) return [];
  const name = (provider.name || '').toLowerCase();
  const base = (provider.base_url || '').toLowerCase();
  const matched = PROVIDER_TEMPLATES.find(t =>
    (t.docUrl || (t.extraDocs && t.extraDocs.length > 0)) && (
      (base && base.includes(t.id)) ||
      (name && name.includes(t.id)) ||
      (t.id === 'lmstudio' && (name.includes('lmstudio') || name.includes('lm studio') || base.includes('1234'))) ||
      (t.id === 'qwen' && (name.includes('qwen') || name.includes('dashscope') || name.includes('tongyi') || base.includes('dashscope') || base.includes('aliyuncs'))) ||
      (t.id === 'glm' && (name.includes('zhipu') || base.includes('bigmodel'))) ||
      (t.id === 'moonshot' && name.includes('kimi')) ||
      (t.id === 'siliconflow' && (name.includes('silicon') || base.includes('siliconflow')))
    )
  );
  if (!matched) return [];
  const list: DocItem[] = [];
  if (matched.docUrl) {
    list.push({ title: `${matched.name} Docs`, url: matched.docUrl });
  }
  if (matched.extraDocs) {
    list.push(...matched.extraDocs);
  }
  return list;
}

export function getProviderDocUrl(provider?: { name?: string; base_url?: string } | null): string | undefined {
  const docs = getProviderDocs(provider);
  return docs.length > 0 ? docs[0].url : undefined;
}

export function isLocalOrModelsFetchable(provider?: { name?: string; base_url?: string } | null): boolean {
  if (!provider) return false;
  const name = (provider.name || '').toLowerCase();
  const base = (provider.base_url || '').toLowerCase();
  return (
    name.includes('lmstudio') ||
    name.includes('lm studio') ||
    base.includes('1234') ||
    name.includes('ollama') ||
    base.includes('11434') ||
    base.includes('localhost') ||
    base.includes('127.0.0.1')
  );
}

export function resolveMaxTokensField(modelName: string): 'max_tokens' | 'max_completion_tokens' {
  const lower = (modelName || '').trim().toLowerCase();
  // OpenAI o1, o3 series reasoning models require max_completion_tokens
  if (lower.startsWith('o1') || lower.startsWith('o3')) {
    return 'max_completion_tokens';
  }
  return 'max_tokens';
}

export function cleanProviderName(name: string): string {
  if (!name) return '';
  return name.replace(/\s*\((?:local|本地)\)/gi, '').trim();
}



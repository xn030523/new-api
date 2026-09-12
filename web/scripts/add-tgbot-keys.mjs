import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const LOCALES_DIR = join(import.meta.dirname, '..', 'src', 'i18n', 'locales')

const TRANSLATIONS = {
  en: {
    'Add Target': 'Add Target',
    'Advanced Settings (JSON)': 'Advanced Settings (JSON)',
    'Chat ID (e.g. -100123456)': 'Chat ID (e.g. -100123456)',
    'Enable Bot': 'Enable Bot',
    Features: 'Features',
    'Get from @BotFather on Telegram': 'Get from @BotFather on Telegram',
    'Global bot configuration': 'Global bot configuration',
    'No targets configured. Add a chat/topic to enable this feature.':
      'No targets configured. Add a chat/topic to enable this feature.',
    'Push Targets': 'Push Targets',
    Stopped: 'Stopped',
    'Telegram Bot': 'Telegram Bot',
    'TG Bot': 'TG Bot',
    'Thread ID (0 = no topic)': 'Thread ID (0 = no topic)',
  },
  zh: {
    'Add Target': '添加推送目标',
    'Advanced Settings (JSON)': '高级设置（JSON）',
    'Chat ID (e.g. -100123456)': '群组 ID（例如 -100123456）',
    'Enable Bot': '启用机器人',
    Features: '功能',
    'Get from @BotFather on Telegram': '在 Telegram 上从 @BotFather 获取',
    'Global bot configuration': '机器人全局配置',
    'No targets configured. Add a chat/topic to enable this feature.':
      '尚未配置推送目标。添加群组/话题后此功能才会生效。',
    'Push Targets': '推送目标',
    Stopped: '已停止',
    'Telegram Bot': 'Telegram 机器人',
    'TG Bot': 'TG 机器人',
    'Thread ID (0 = no topic)': '话题 ID（0 表示不使用话题）',
  },
  'zh-TW': {
    'Add Target': '新增推送目標',
    'Advanced Settings (JSON)': '進階設定（JSON）',
    'Chat ID (e.g. -100123456)': '群組 ID（例如 -100123456）',
    'Enable Bot': '啟用機器人',
    Features: '功能',
    'Get from @BotFather on Telegram': '在 Telegram 上向 @BotFather 取得',
    'Global bot configuration': '機器人全域設定',
    'No targets configured. Add a chat/topic to enable this feature.':
      '尚未設定推送目標。新增群組/話題後此功能才會生效。',
    'Push Targets': '推送目標',
    Stopped: '已停止',
    'Telegram Bot': 'Telegram 機器人',
    'TG Bot': 'TG 機器人',
    'Thread ID (0 = no topic)': '話題 ID（0 表示不使用話題）',
  },
  ja: {
    'Add Target': '送信先を追加',
    'Advanced Settings (JSON)': '詳細設定（JSON）',
    'Chat ID (e.g. -100123456)': 'チャット ID（例: -100123456）',
    'Enable Bot': 'ボットを有効化',
    Features: '機能',
    'Get from @BotFather on Telegram': 'Telegram の @BotFather から取得',
    'Global bot configuration': 'ボットのグローバル設定',
    'No targets configured. Add a chat/topic to enable this feature.':
      '送信先が未設定です。チャット／トピックを追加するとこの機能が有効になります。',
    'Push Targets': '送信先',
    Stopped: '停止中',
    'Telegram Bot': 'Telegram ボット',
    'TG Bot': 'TG ボット',
    'Thread ID (0 = no topic)': 'スレッド ID（0 はトピックなし）',
  },
  ru: {
    'Add Target': 'Добавить получателя',
    'Advanced Settings (JSON)': 'Расширенные настройки (JSON)',
    'Chat ID (e.g. -100123456)': 'ID чата (например, -100123456)',
    'Enable Bot': 'Включить бота',
    Features: 'Функции',
    'Get from @BotFather on Telegram': 'Получите у @BotFather в Telegram',
    'Global bot configuration': 'Общая конфигурация бота',
    'No targets configured. Add a chat/topic to enable this feature.':
      'Получатели не настроены. Добавьте чат или тему, чтобы включить эту функцию.',
    'Push Targets': 'Получатели',
    Stopped: 'Остановлен',
    'Telegram Bot': 'Telegram-бот',
    'TG Bot': 'TG-бот',
    'Thread ID (0 = no topic)': 'ID темы (0 — без темы)',
  },
  fr: {
    'Add Target': 'Ajouter une cible',
    'Advanced Settings (JSON)': 'Paramètres avancés (JSON)',
    'Chat ID (e.g. -100123456)': 'ID de discussion (ex. -100123456)',
    'Enable Bot': 'Activer le bot',
    Features: 'Fonctionnalités',
    'Get from @BotFather on Telegram': 'À obtenir auprès de @BotFather sur Telegram',
    'Global bot configuration': 'Configuration globale du bot',
    'No targets configured. Add a chat/topic to enable this feature.':
      'Aucune cible configurée. Ajoutez une discussion ou un sujet pour activer cette fonctionnalité.',
    'Push Targets': 'Cibles de diffusion',
    Stopped: 'Arrêté',
    'Telegram Bot': 'Bot Telegram',
    'TG Bot': 'Bot TG',
    'Thread ID (0 = no topic)': 'ID de sujet (0 = aucun sujet)',
  },
  vi: {
    'Add Target': 'Thêm đích gửi',
    'Advanced Settings (JSON)': 'Cài đặt nâng cao (JSON)',
    'Chat ID (e.g. -100123456)': 'ID nhóm chat (ví dụ -100123456)',
    'Enable Bot': 'Bật bot',
    Features: 'Tính năng',
    'Get from @BotFather on Telegram': 'Lấy từ @BotFather trên Telegram',
    'Global bot configuration': 'Cấu hình chung của bot',
    'No targets configured. Add a chat/topic to enable this feature.':
      'Chưa cấu hình đích gửi. Thêm nhóm chat/chủ đề để bật tính năng này.',
    'Push Targets': 'Đích gửi',
    Stopped: 'Đã dừng',
    'Telegram Bot': 'Bot Telegram',
    'TG Bot': 'Bot TG',
    'Thread ID (0 = no topic)': 'ID chủ đề (0 = không dùng chủ đề)',
  },
}

let total = 0
for (const [locale, entries] of Object.entries(TRANSLATIONS)) {
  const file = join(LOCALES_DIR, `${locale}.json`)
  const data = JSON.parse(readFileSync(file, 'utf8'))
  let added = 0
  for (const [key, value] of Object.entries(entries)) {
    if (data.translation[key] === undefined) {
      data.translation[key] = value
      added += 1
    }
  }
  const sorted = {}
  for (const key of Object.keys(data.translation).sort((a, b) => a.localeCompare(b))) {
    sorted[key] = data.translation[key]
  }
  data.translation = sorted
  writeFileSync(file, JSON.stringify(data, null, 2) + '\n', 'utf8')
  console.log(`${locale}: +${added}`)
  total += added
}
console.log(`total: ${total}`)

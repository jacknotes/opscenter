// 构建期版本号解析：优先 git describe，取不到 tag 时用 package.json 版本合成同格式的版本号。
// 通过 Vite 虚拟模块 `virtual:app-version` 提供给前端（dev 与 build 均可用），
// 由 src/utils/version.js 读取展示。
import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const WEB_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..')

/** 虚拟模块 id，前端通过 import { APP_VERSION } from 'virtual:app-version' 使用 */
export const VIRTUAL_MODULE_ID = 'virtual:app-version'
const RESOLVED_VIRTUAL_MODULE_ID = '\0' + VIRTUAL_MODULE_ID

// semver tag 描述：v2.0.1 / v2.0.1-7-g401385d / v2.0.1-7-g401385d-dirty
const TAG_DESCRIBE_PATTERN = /^v?\d+(?:\.\d+)+(?:-\d+-g[0-9a-f]+)?(?:-dirty)?$/
const DIRTY_SUFFIX = /-dirty$/
const BARE_SHA_PATTERN = /^[0-9a-f]{4,40}$/

function runGit(args, cwd) {
  try {
    const out = execFileSync('git', args, {
      cwd,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
      timeout: 5000,
    })
    return (out || '').trim() || null
  } catch {
    // 不在 git 仓库、未安装 git、超时等一律视为不可用，交由回退逻辑处理
    return null
  }
}

/**
 * 把 git describe 的输出规范化为展示用版本号，尽量保持 `v<版本>-<提交数>-g<哈希>` 格式。
 *
 * - 有可达的 semver tag：直接采用 describe 结果（v2.0.1-7-g401385d），补全 v 前缀
 * - 无可达 tag（describe 退化为裸哈希）：用 package.json 版本作基线，配上总提交数，
 *   合成 v<package.json 版本>-<提交数>-g<hash>，而不是展示一串裸哈希
 * - git 完全不可用（如 Docker 构建）：只展示 v<package.json 版本>
 * - 工作区有未提交改动：追加 -dirty
 */
export function formatVersion(describe, commitCount, pkgVersion) {
  const desc = (describe || '').trim()
  const base = (pkgVersion || '').trim() || '0.0.0'

  if (!desc) return `v${base}`

  if (TAG_DESCRIBE_PATTERN.test(desc)) return `v${desc.replace(/^v/, '')}`

  const dirty = DIRTY_SUFFIX.test(desc)
  const core = desc.replace(DIRTY_SUFFIX, '')
  const suffix = dirty ? '-dirty' : ''

  // 无 tag 时的裸哈希：用 package.json 版本 + 总提交数补出标准 describe 形态
  if (BARE_SHA_PATTERN.test(core) && commitCount) return `v${base}-${commitCount}-g${core}${suffix}`

  // 其余异常形态（非 semver tag、tag 名不规范等）：原样保留，便于排查
  return `v${base}+${core}${dirty ? '.dirty' : ''}`
}

/** 读取 package.json 的 version 字段；失败时返回空串，由 formatVersion 兜底。 */
export function readPackageVersion(webRoot = WEB_ROOT) {
  try {
    return JSON.parse(readFileSync(join(webRoot, 'package.json'), 'utf8')).version || ''
  } catch {
    return ''
  }
}

/** 读取 HEAD 的总提交数，用于无 tag 时合成 describe 的 -N- 段。 */
export function readCommitCount(cwd = WEB_ROOT) {
  const out = runGit(['rev-list', '--count', 'HEAD'], cwd)
  const n = Number.parseInt(out || '', 10)
  return Number.isFinite(n) && n > 0 ? n : null
}

/** 执行 git describe --tags --long --always --dirty；失败返回 null。 */
export function readGitDescribe(cwd = WEB_ROOT) {
  return runGit(['describe', '--tags', '--long', '--always', '--dirty'], cwd)
}

/**
 * 解析当前构建应使用的版本号。
 * 环境变量 APP_VERSION / VITE_APP_VERSION 优先（供 Docker/CI 在无 .git 时显式注入）。
 */
export function resolveVersion({ cwd = WEB_ROOT, env = process.env } = {}) {
  const override = (env.APP_VERSION || env.VITE_APP_VERSION || '').trim()
  if (override) return override

  const describe = readGitDescribe(cwd)
  const needsCount = describe !== null && !TAG_DESCRIBE_PATTERN.test(describe)
  // package.json 始终从 web 根目录读，与 git cwd 无关
  return formatVersion(describe, needsCount ? readCommitCount(cwd) : null, readPackageVersion())
}

/**
 * Vite 插件：把版本号作为虚拟模块暴露给前端。
 *
 * 用虚拟模块而非 `define`，是因为 Vite 的 define 在 dev 模式不生效
 * （vite:define 插件在非 build 时直接 return），会导致开发环境永远显示兜底值。
 */
export function appVersionPlugin(version = resolveVersion()) {
  return {
    name: 'opscenter:app-version',
    resolveId(id) {
      if (id === VIRTUAL_MODULE_ID) return RESOLVED_VIRTUAL_MODULE_ID
      return null
    },
    load(id) {
      if (id === RESOLVED_VIRTUAL_MODULE_ID) {
        return `// 由 build/appVersion.js 在构建/启动时生成\nexport const APP_VERSION = ${JSON.stringify(version)}\n`
      }
      return null
    },
  }
}

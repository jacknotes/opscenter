/**
 * 应用版本号。
 *
 * 值由构建期解析后经 Vite 虚拟模块 `virtual:app-version` 提供
 * （见 web/build/appVersion.js）：优先 git describe 输出（如 v2.0.1-7-g401385d），
 * 取不到 tag 时用 package.json 版本合成同格式。若在非 Vite 环境（如单元测试）引入，
 * 虚拟模块会被替换为 v0.0.0。
 */
import { APP_VERSION } from 'virtual:app-version'

/** 展示用应用名 */
export const APP_NAME = 'OpsCenter'

export { APP_VERSION }

/** 侧边栏底部展示用的完整标识，如 OpsCenter · v2.0.1-7-g401385d */
export const APP_SIGNATURE = `${APP_NAME} · ${APP_VERSION}`

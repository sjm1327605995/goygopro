import classicSleeve from '../../textures/cover.jpg';
import circuitSleeve from '../../textures/sleeve-circuit.svg';
import royalSleeve from '../../textures/sleeve-royal.svg';
import observatory from '../../textures/bg.jpg';
import nebula from '../../textures/arena-nebula.svg';

export const SLEEVES = [
  {id: 'classic', name: '经典漩涡', url: classicSleeve},
  {id: 'circuit', name: '星轨回路', url: circuitSleeve},
  {id: 'royal', name: '曜金纹章', url: royalSleeve},
];
export const BACKGROUNDS = [
  {id: 'arena', name: '苍石遗迹', url: ''},
  {id: 'nebula', name: '星海边界', url: nebula},
  {id: 'observatory', name: '决斗观景台', url: observatory},
];
export interface Appearance {
  sleeve: string;
  background: string;
  customSleeve: string;
  customBackground: string;
  sleeveName: string;
  backgroundName: string;
  environmentOpacity: number;
}
export const APPEARANCE_DEFAULTS: Appearance = {
  sleeve: 'classic', background: 'arena', customSleeve: '', customBackground: '',
  sleeveName: '', backgroundName: '', environmentOpacity: 65,
};
const KEY = 'goygopro.appearance.v1';
const imageData = (value: unknown): string => typeof value === 'string' && value.length <= 2_000_000 && /^data:image\/(jpeg|png|webp);base64,[A-Za-z0-9+/=]+$/.test(value) ? value : '';
function readAppearance(): Appearance {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) || '{}');
    const customSleeve = imageData(raw.customSleeve), customBackground = imageData(raw.customBackground);
    return {
      sleeve: SLEEVES.some(item => item.id === raw.sleeve) || (raw.sleeve === 'custom' && customSleeve) ? raw.sleeve : 'classic',
      background: BACKGROUNDS.some(item => item.id === raw.background) || (raw.background === 'custom' && customBackground) ? raw.background : 'arena',
      customSleeve, customBackground,
      sleeveName: typeof raw.sleeveName === 'string' ? raw.sleeveName.slice(0, 100) : '',
      backgroundName: typeof raw.backgroundName === 'string' ? raw.backgroundName.slice(0, 100) : '',
      environmentOpacity: Number.isFinite(raw.environmentOpacity) ? Math.max(20, Math.min(85, raw.environmentOpacity)) : 65,
    };
  } catch { return {...APPEARANCE_DEFAULTS}; }
}

// Cosmetic preferences live on this device in both browser and Wails WebView.
// Images never enter system.conf or the duel protocol.
class AppearanceStore {
  private value = readAppearance();
  private listeners = new Set<() => void>();
  getSnapshot = () => this.value;
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; };
  set = (patch: Partial<Appearance>) => {
    const next = {...this.value, ...patch};
    try { localStorage.setItem(KEY, JSON.stringify(next)); }
    catch { throw new Error('无法保存外观：本地存储不可用或空间不足。请移除自定义图片后重试。'); }
    this.value = next;
    this.listeners.forEach(listener => listener());
  };
  reset = () => this.set({...APPEARANCE_DEFAULTS});
}
export const appearanceStore = new AppearanceStore();
export const sleeveURL = (value: Appearance) => value.sleeve === 'custom' ? value.customSleeve : SLEEVES.find(item => item.id === value.sleeve)?.url || SLEEVES[0].url;
export const backgroundURL = (value: Appearance) => value.background === 'custom' ? value.customBackground : BACKGROUNDS.find(item => item.id === value.background)?.url || '';

export async function prepareAppearanceImage(file: File, kind: 'sleeve' | 'background'): Promise<string> {
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) throw new Error('请选择 PNG、JPG 或 WebP 图片。');
  if (file.size > 12 * 1024 * 1024) throw new Error('图片不能超过 12 MB。');
  const url = URL.createObjectURL(file);
  try {
    const image = new Image();
    image.src = url;
    await image.decode().catch(() => { throw new Error('无法读取这张图片，请更换文件。'); });
    if (!image.naturalWidth || image.naturalWidth * image.naturalHeight > 24_000_000) throw new Error('图片分辨率过大，请缩小后重试。');
    const canvas = document.createElement('canvas');
    canvas.width = kind === 'sleeve' ? 512 : 1600;
    canvas.height = kind === 'sleeve' ? 744 : 900;
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = '#0a1420'; ctx.fillRect(0, 0, canvas.width, canvas.height);
    const scale = Math.max(canvas.width / image.naturalWidth, canvas.height / image.naturalHeight);
    const width = image.naturalWidth * scale, height = image.naturalHeight * scale;
    ctx.drawImage(image, (canvas.width - width) / 2, (canvas.height - height) / 2, width, height);
    const result = canvas.toDataURL('image/jpeg', .86);
    if (!imageData(result)) throw new Error('图片处理后仍然过大，请选择更简单的图片。');
    return result;
  } finally { URL.revokeObjectURL(url); }
}

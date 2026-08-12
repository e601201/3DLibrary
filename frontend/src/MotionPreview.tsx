// design/Design.pen 画面02 のモーションプレビュー(CONTEXT.md「モーション
// プレビュー」)。GLB ビューワに重なる、シェイプキーとクリップの操作 UI。
// 3D 表示の上に乗るので、配色は stage-* 固定。
//
// three との受け渡しは GlbViewer の ViewerApi が持ち、こちらは値を映して
// 操作を返すだけに徹する(メッシュ実体には触れない)。

import { Pause, Play, Repeat, RotateCcw, SkipBack, StepBack, StepForward } from 'lucide-react';
import { cx, type LucideIcon } from './ui';

// GLB から読み取ったモーション要素。構造は読み込み時に一度だけ組み立てる。
export type ShapeKey = {
  objectName: string; // グループ見出しに使うノード名
  name: string; // シェイプキー名
  defaultValue: number; // エクスポート時の値(リセット先)
};

export type Motion = {
  clips: { name: string; duration: number }[];
  shapeKeys: ShapeKey[];
  // クリップごとに「そのクリップが動かすシェイプキー」の真偽値。glTF は
  // ノードの weights をまとめて書くので、駆動はメッシュ単位で効く
  drivenByClip: boolean[][];
};

// フレームレートを知っているかで、刻みも表記も変わる。分岐があちこちへ
// 散らないよう、時間の扱いはこの型に集める。フレームレートは抽出メタデータ
// から来るので、モーションプレビュー導入前のキャッシュでは未知になる。
export type Timebase = {
  knowsFrames: boolean;
  seekStep: number; // シークバーの刻み(秒)
  step: (time: number, frames: number) => number; // n フレーム送った時刻
  playhead: (time: number, duration: number) => string;
};

export function timebaseOf(frameRate: number | null): Timebase {
  if (frameRate === null) {
    return {
      knowsFrames: false,
      seekStep: 0.01,
      step: (time) => time,
      playhead: (time, duration) => `${time.toFixed(2)}s / ${duration.toFixed(2)}s`,
    };
  }
  // Blender の glTF エクスポータは「フレーム番号 ÷ fps」を時刻に書くので、
  // 秒にフレームレートを掛け戻すと Blender のフレーム番号がそのまま出る
  const frames = (seconds: number) => Math.round(seconds * frameRate);
  return {
    knowsFrames: true,
    seekStep: 1 / frameRate,
    // 秒に足すと端数が溜まるので、フレーム番号を起点に数える
    step: (time, delta) => (frames(time) + delta) / frameRate,
    playhead: (time, duration) => {
      const total = frames(duration);
      const current = Math.min(frames(time), total);
      const width = Math.max(3, String(total).length);
      const fps = Number.isInteger(frameRate) ? String(frameRate) : frameRate.toFixed(2);
      return `F ${String(current).padStart(width, '0')} / ${total} · ${fps} FPS`;
    },
  };
}

// クリップの長さは「どれくらいの尺か」なので、再生位置と違って秒で見せる
// (design/Design.pen 画面02 のクリップ一覧)
function clipLength(duration: number) {
  return `${duration.toFixed(1)}s`;
}

// ビューポート左上のパネル。シェイプキーのスライダーとクリップ一覧を持つ
export function MotionPanel({
  motion,
  influences,
  driven,
  clipIndex,
  playing,
  onSelectClip,
  onInfluence,
  onReset,
}: {
  motion: Motion;
  influences: number[];
  driven: boolean[]; // 選択中のクリップが動かすシェイプキー
  clipIndex: number;
  playing: boolean;
  onSelectClip: (index: number) => void;
  onInfluence: (index: number, value: number) => void;
  onReset: () => void;
}) {
  const groups = groupByObject(motion.shapeKeys);
  return (
    <div className="pointer-events-auto flex max-h-[22rem] w-64 flex-col gap-3 overflow-y-auto border border-stage-border bg-stage/90 p-3 backdrop-blur-sm">
      {motion.shapeKeys.length > 0 && (
        <section className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-2">
            <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
              SHAPE KEYS <span className="text-stage-accent">{motion.shapeKeys.length}</span>
            </p>
            <button
              type="button"
              onClick={onReset}
              title="Blender で保存した値に戻す"
              className="flex items-center gap-1 font-mono text-[10px] leading-none text-stage-ink-muted transition hover:text-stage-ink"
            >
              <RotateCcw size={11} />
              リセット
            </button>
          </div>
          {groups.map((group) => (
            <div key={group.objectName} className="flex flex-col gap-2">
              {/* オブジェクトが 1 つだけなら見出しは邪魔なので出さない */}
              {groups.length > 1 && (
                <p
                  className="truncate font-mono text-[10px] leading-none text-stage-ink-faint"
                  title={group.objectName}
                >
                  {group.objectName}
                </p>
              )}
              {group.keys.map(({ index, name }) => (
                <ShapeKeySlider
                  key={index}
                  name={name}
                  value={influences[index] ?? 0}
                  // 再生中はクリップが毎フレーム上書きするので操作させない
                  driven={playing && (driven[index] ?? false)}
                  onChange={(value) => onInfluence(index, value)}
                />
              ))}
            </div>
          ))}
        </section>
      )}

      {motion.clips.length > 0 && (
        <section className="flex flex-col gap-1.5">
          {/* 件数はアクセント色で右端に置く(design/Design.pen 画面02) */}
          <div className="flex items-baseline justify-between gap-2">
            <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
              ANIMATION CLIPS
            </p>
            <p className="font-mono text-[10px] leading-none text-stage-accent">
              {motion.clips.length}
            </p>
          </div>
          <div className="flex flex-col">
            {motion.clips.map((clip, index) => {
              const selected = index === clipIndex;
              const active = selected && playing;
              return (
                <button
                  key={`${clip.name}-${index}`}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => onSelectClip(index)}
                  title={selected ? (playing ? '一時停止' : '再生') : `${clip.name} を再生`}
                  className={cx(
                    'flex items-center gap-2 px-2 py-1.5 text-left transition',
                    // アクセント色は「いま動いている」印。停止中の選択は
                    // 一段控えめにして、タイムラインが指すクリップだと分かればよい
                    active
                      ? 'bg-stage-accent/15 text-stage-accent'
                      : selected
                        ? 'text-stage-ink'
                        : 'text-stage-ink-muted hover:text-stage-ink',
                  )}
                >
                  {active ? (
                    <Pause size={12} className="shrink-0" />
                  ) : (
                    <Play size={12} className="shrink-0" />
                  )}
                  <span className="truncate font-mono text-[11px] leading-none">{clip.name}</span>
                  <span className="ml-auto shrink-0 font-mono text-[10px] leading-none opacity-70">
                    {clipLength(clip.duration)}
                  </span>
                </button>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

// 効いているキーは値をアクセント色に、0 のキーはミュート色にして、
// どのキーが動いているかを値の色だけで拾えるようにする(名前は常に同じ明度)
function ShapeKeySlider({
  name,
  value,
  driven,
  onChange,
}: {
  name: string;
  value: number;
  driven: boolean;
  onChange: (value: number) => void;
}) {
  const muted = value < 0.005;
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-baseline justify-between gap-2">
        <span className="truncate font-mono text-[11px] leading-none text-stage-ink" title={name}>
          {name}
        </span>
        <span
          className={cx(
            'shrink-0 font-mono text-[10px] leading-none',
            muted ? 'text-stage-ink-faint' : 'text-stage-accent',
          )}
        >
          {value.toFixed(2)}
        </span>
      </div>
      <input
        type="range"
        min={0}
        max={1}
        step={0.01}
        value={value}
        disabled={driven}
        aria-label={name}
        title={driven ? '再生中のクリップが動かしています' : undefined}
        onChange={(e) => onChange(Number(e.target.value))}
        className={cx('w-full accent-stage-accent', driven && 'cursor-not-allowed opacity-40')}
      />
    </div>
  );
}

// ビューポート下部の再生タイムライン。クリップのある GLB でだけ出す
export function TimelineBar({
  clipName,
  time,
  duration,
  playing,
  loop,
  timebase,
  onTogglePlay,
  onRewind,
  onStep,
  onSeek,
  onToggleLoop,
}: {
  clipName: string;
  time: number;
  duration: number;
  playing: boolean;
  loop: boolean;
  timebase: Timebase;
  onTogglePlay: () => void;
  onRewind: () => void;
  onStep: (frames: number) => void;
  onSeek: (seconds: number) => void;
  onToggleLoop: () => void;
}) {
  // フレーム送りはフレームレートを知らないと刻めない。旧キャッシュでも
  // ボタンは残し、再生成すれば使えることを説明で伝える
  const stepReason = timebase.knowsFrames
    ? null
    : 'フレームレートが分からないので送れません(再生成すると使えます)';
  return (
    <div className="pointer-events-auto flex w-full max-w-2xl items-center gap-1.5 border border-stage-border bg-stage/80 px-2.5 py-[6px] backdrop-blur-sm">
      <TimelineButton icon={SkipBack} label="先頭に戻す" onClick={onRewind} />
      <TimelineButton
        icon={StepBack}
        label={stepReason ?? '1 フレーム戻す'}
        disabled={!timebase.knowsFrames}
        onClick={() => onStep(-1)}
      />
      <TimelineButton
        icon={playing ? Pause : Play}
        label={playing ? '一時停止' : '再生'}
        onClick={onTogglePlay}
      />
      <TimelineButton
        icon={StepForward}
        label={stepReason ?? '1 フレーム進める'}
        disabled={!timebase.knowsFrames}
        onClick={() => onStep(1)}
      />
      <span
        className="max-w-[7rem] shrink-0 truncate font-mono text-[10px] leading-none text-stage-ink-muted"
        title={clipName}
      >
        {clipName}
      </span>
      <input
        type="range"
        min={0}
        max={duration}
        step={timebase.seekStep}
        value={Math.min(time, duration)}
        aria-label="再生位置"
        onChange={(e) => onSeek(Number(e.target.value))}
        className="min-w-0 flex-1 accent-stage-accent"
      />
      <span className="shrink-0 font-mono text-[10px] leading-none text-stage-ink-faint">
        {timebase.playhead(time, duration)}
      </span>
      {/* ループは再生位置を進める操作ではなく持続する設定なので、
          送り・戻しの並びから離して右端に置く */}
      <TimelineButton icon={Repeat} label="ループ再生の切替" active={loop} onClick={onToggleLoop} />
    </div>
  );
}

// タイムラインの小さな操作ボタン(枠なし。ツール列より一段軽い見た目)
function TimelineButton({
  icon: Icon,
  label,
  active,
  disabled = false,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  active?: boolean; // トグルのときだけ渡す(押下状態を読み上げに出す)
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={active}
      disabled={disabled}
      onClick={onClick}
      className={cx(
        'shrink-0 p-1 transition',
        active ? 'text-stage-accent' : 'text-stage-ink-muted',
        disabled ? 'cursor-not-allowed opacity-40' : 'hover:text-stage-ink',
      )}
    >
      <Icon size={13} />
    </button>
  );
}

// シェイプキーを持つオブジェクトごとにまとめる(並び順は元のままで、
// 同じオブジェクトのキーが 1 か所に集まる)
function groupByObject(shapeKeys: ShapeKey[]) {
  const groups: { objectName: string; keys: { index: number; name: string }[] }[] = [];
  shapeKeys.forEach((key, index) => {
    let group = groups.find((g) => g.objectName === key.objectName);
    if (!group) {
      group = { objectName: key.objectName, keys: [] };
      groups.push(group);
    }
    group.keys.push({ index, name: key.name });
  });
  return groups;
}

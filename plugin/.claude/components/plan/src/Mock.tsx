import { useState } from 'react';
import { renderInlineMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';
import { setComment } from './logic';
import type { MockSpec } from './logic';
import { MockFrame } from './MockFrame';

interface Pin {
  ref: string;
  title: string;
  body?: string;
}

interface MockBlock extends MockSpec {
  caption?: string;
  pins?: Pin[];
}

export function Mock({ block, value, submit, disabled }: PackComponentProps) {
  const b = block as unknown as MockBlock;
  const pins = b.pins ?? [];
  const comments = (value as { comments: Record<string, string> } | undefined)?.comments ?? {};
  const [active, setActive] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const chromeTitle = b.frame === 'desktop' || b.frame === 'terminal';

  const open = (ref: string) => {
    setActive(active === ref ? null : ref);
    setDraft(comments[ref] ?? '');
  };
  const save = (ref: string) => {
    submit({ comments: setComment(comments, ref, draft) });
    setActive(null);
  };
  const activePin = pins.find((p) => p.ref === active);

  return (
    <section className="ccpl-frame">
      {b.title && !chromeTitle && <header className="ccpl-title">{b.title}</header>}
      <MockFrame
        spec={b}
        pinRefs={pins.map((p) => p.ref)}
        noted={new Set(Object.keys(comments))}
        active={active}
        onPin={open}
        popover={
          activePin && (
            <div className="ccpl-pop" role="note">
              <div className="ccpl-pop-title">
                <span className="ccpl-pin-n">{pins.indexOf(activePin) + 1}</span>
                {activePin.title}
              </div>
              {activePin.body && <div className="ccpl-pop-body" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(activePin.body) }} />}
            </div>
          )
        }
      />
      {pins.length > 0 && (
        <ol className="ccpl-pins">
          {pins.map((p, i) => (
            <li key={p.ref} className={p.ref === active ? 'ccpl-on' : undefined}>
              <button type="button" className="ccpl-pin-n" onClick={() => open(p.ref)} aria-expanded={p.ref === active}>
                {i + 1}
              </button>
              <div className="ccpl-pin-text">
                <div>
                  <b>{p.title}</b>
                  {p.body && (
                    <>
                      {' — '}
                      <span dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(p.body) }} />
                    </>
                  )}
                </div>
                {comments[p.ref] && p.ref !== active && <div className="ccpl-pin-note">{comments[p.ref]}</div>}
                {p.ref === active && !disabled && (
                  <div className="ccpl-editor">
                    <textarea
                      rows={2}
                      value={draft}
                      placeholder="Comment on this part of the mock"
                      onChange={(e) => setDraft(e.target.value)}
                    />
                    <div className="ccpl-actions">
                      <button type="button" onClick={() => setActive(null)}>
                        Cancel
                      </button>
                      <button type="button" className="ccpl-primary" onClick={() => save(p.ref)}>
                        {draft.trim() || !comments[p.ref] ? 'Save' : 'Remove'}
                      </button>
                    </div>
                  </div>
                )}
                {p.ref === active && disabled && comments[p.ref] && <div className="ccpl-pin-note">{comments[p.ref]}</div>}
              </div>
            </li>
          ))}
        </ol>
      )}
      {b.caption && <div className="ccpl-caption" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(b.caption) }} />}
    </section>
  );
}

import type { Record as RecordBlock } from '../schema';

export function RecordView({ block }: { block: RecordBlock }) {
  return (
    <section className="record-block">
      {block.title ? <h4 className="record-title">{block.title}</h4> : null}
      {block.chips && block.chips.length > 0 ? (
        <span className="chips">
          {block.chips.map((chip, i) => (
            <span key={i} className={`chip chip-${chip.tone ?? 'default'}`}>
              {chip.label}
            </span>
          ))}
        </span>
      ) : null}
      <dl className="record-facts">
        {block.facts.map((fact, i) => (
          <div key={i} className={`fact fact-${fact.tone ?? 'default'}`}>
            <dt className="fact-label">{fact.label}</dt>
            <dd className="fact-value">{fact.value}</dd>
          </div>
        ))}
      </dl>
      {block.links && block.links.length > 0 ? (
        <div className="record-links">
          {block.links.map((link, i) => (
            <a key={i} href={link.url} target="_blank" rel="noopener noreferrer">
              {link.label}
            </a>
          ))}
        </div>
      ) : null}
    </section>
  );
}

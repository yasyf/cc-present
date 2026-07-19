import type { Record as RecordBlock } from '../schema';

// Placeholder Record renderer — Phase 1 replaces this body with the styled profile
// (chips, toned fact grid, https link row). It renders the facts so the block
// dispatches and typechecks. Aliased import dodges the `Record` utility-type name.
export function RecordView({ block }: { block: RecordBlock }) {
  return (
    <section className="record-block">
      {block.title ? <h4 className="record-title">{block.title}</h4> : null}
      <dl className="record-facts">
        {block.facts.map((fact, i) => (
          <div key={i}>
            <dt>{fact.label}</dt>
            <dd>{fact.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

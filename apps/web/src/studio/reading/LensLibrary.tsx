import { groupReadingDeck, readingDeck, type ReadingDeckEntry } from "./readingDeck";

export type LensLibraryProps = {
  onPick: (cardId: string) => void;
  onClose: () => void;
};

function LensRow({ entry, onPick }: { entry: ReadingDeckEntry; onPick: (cardId: string) => void }) {
  return (
    <button type="button" className="mk-lens-library__row" onClick={() => onPick(entry.id)}>
      <span className="mk-lens-library__row-name">{entry.name}</span>
      <span className="mk-lens-library__row-purpose">{entry.purpose}</span>
    </button>
  );
}

// LensLibrary (透镜库) — lets the student browse the whole reading deck and
// summon a CHOSEN card onto the article herself, rather than only ever
// waiting for the AI to propose one. A calm modal/panel over the reading
// room; picking a row hands the card id to the caller (ReadingRoom wires
// this straight into loop.summonCard) and the caller is responsible for
// closing the panel — this component itself holds no open/closed state.
export function LensLibrary({ onPick, onClose }: LensLibraryProps) {
  const { sourceCheck, families } = groupReadingDeck(readingDeck());

  return (
    <div className="mk-lens-library__backdrop" role="presentation" onClick={onClose}>
      <div
        className="mk-lens-library"
        role="dialog"
        aria-modal="true"
        aria-label="透镜库"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="mk-lens-library__head">
          <div>
            <span className="mk-lens-library__kicker">透镜库</span>
            <h2>挑一副透镜，换个角度读这篇文章</h2>
          </div>
          <button type="button" className="mk-lens-library__close" aria-label="关闭透镜库" onClick={onClose}>
            ×
          </button>
        </header>

        <div className="mk-lens-library__body">
          {sourceCheck.length > 0 && (
            <section className="mk-lens-library__group">
              <h3>信源体检</h3>
              <div className="mk-lens-library__rows">
                {sourceCheck.map((entry) => (
                  <LensRow key={entry.id} entry={entry} onPick={onPick} />
                ))}
              </div>
            </section>
          )}
          {families.map((group) => (
            <section key={group.key} className="mk-lens-library__group">
              <h3>{group.label}</h3>
              <div className="mk-lens-library__rows">
                {group.entries.map((entry) => (
                  <LensRow key={entry.id} entry={entry} onPick={onPick} />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}

import '@fontsource-variable/lora';
import './sheets.css';

// A long category name gets a smaller font so it stays on one or two lines.
function titleSize(title) {
    const n = title.length;
    return n <= 14 ? 40 : n <= 22 ? 34 : n <= 32 ? 28 : 23;
}

// Many answers get smaller numbers and, past 18, two columns.
function lineLayout(n) {
    const cols = n > 18 ? 2 : 1;
    const rows = Math.ceil(n / cols);
    const font = rows <= 8 ? 30 : rows <= 12 ? 24 : 18;
    return { cols, rows, font };
}

// Sheet is one team's answer sheet for a category: the round number, the
// category and its scoring rules, a line for the team's name (every team gets
// the same sheet) and one numbered line per song.
export default function Sheet({ header, number, category }) {
    const n = Math.max(category.items.length, 1);
    const { cols, rows, font } = lineLayout(n);
    return (
        <div className="sheet">
            <div className="sheet-header">
                {header ? `${header}. ` : ''}Тур №{number}
            </div>
            <div className="sheet-title" style={{ fontSize: `${titleSize(category.title)}pt` }}>
                {category.title}
            </div>
            {category.criteria && <div className="sheet-criteria">{category.criteria}</div>}
            <div className="sheet-team">Назва команди:</div>
            <div className="sheet-team-line" />
            <div className="sheet-answers">Відповіді</div>
            <div className="sheet-lines" style={{ '--rows': rows, gridTemplateColumns: `repeat(${cols}, 1fr)`, fontSize: `${font}pt` }}>
                {Array.from({ length: n }, (_, i) => (
                    <div key={i} className="sheet-line">
                        <span>{i + 1}.</span>
                        <i />
                    </div>
                ))}
            </div>
        </div>
    );
}

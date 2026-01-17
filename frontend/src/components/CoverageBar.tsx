import type { CoverageStats } from '../types'

interface Props {
    coverage: CoverageStats
    height?: number
}

export function CoverageBar({ coverage, height = 8 }: Props) {
    return (
        <div className="w-full">
            <div
                className="bg-slate-100 rounded-full overflow-hidden flex"
                style={{ height: `${height}px` }}
            >
                <div
                    className="bg-green-500 transition-all duration-500"
                    style={{ width: `${coverage.numericPercent}%` }}
                    title={`Numeric: ${coverage.numericPercent.toFixed(1)}%`}
                />
                <div
                    className="bg-purple-500 transition-all duration-500"
                    style={{ width: `${coverage.symbolicPercent}%` }}
                    title={`Symbolic: ${coverage.symbolicPercent.toFixed(1)}%`}
                />
                <div
                    className="bg-slate-400 transition-all duration-500"
                    style={{ width: `${coverage.indirectPercent}%` }}
                    title={`Indirect: ${coverage.indirectPercent.toFixed(1)}%`}
                />
                <div
                    className="bg-red-400 transition-all duration-500"
                    style={{ width: `${coverage.unsupportedPercent}%` }}
                    title={`Unsupported: ${coverage.unsupportedPercent.toFixed(1)}%`}
                />
            </div>
        </div>
    )
}

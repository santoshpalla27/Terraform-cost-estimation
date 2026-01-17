import type { CoverageStats } from '../types'

interface Props {
    totalCost: number
    confidence: number
    coverage: CoverageStats
}

export function CostSummaryCard({ totalCost, confidence, coverage }: Props) {
    const confidenceColor = confidence >= 0.8 ? 'text-green-600' : confidence >= 0.6 ? 'text-yellow-600' : 'text-red-600'

    return (
        <div className="bg-white rounded-2xl shadow-lg border border-slate-200 p-8">
            {/* Total Cost */}
            <div className="text-center mb-8">
                <div className="text-sm font-medium text-slate-500 uppercase tracking-wide">Total Monthly Cost</div>
                <div className="text-5xl font-bold text-slate-900 mt-2">
                    ${totalCost.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </div>
            </div>

            {/* Confidence */}
            <div className="flex items-center justify-between mb-6">
                <span className="text-sm font-medium text-slate-600">Confidence</span>
                <span className={`text-2xl font-bold ${confidenceColor}`}>
                    {(confidence * 100).toFixed(0)}%
                </span>
            </div>

            {/* Coverage Bar */}
            <div className="mb-4">
                <div className="flex items-center justify-between text-xs text-slate-500 mb-2">
                    <span>Coverage Breakdown</span>
                </div>
                <div className="h-4 bg-slate-100 rounded-full overflow-hidden flex">
                    <div
                        className="bg-green-500 h-full transition-all duration-500"
                        style={{ width: `${coverage.numericPercent}%` }}
                        title={`Numeric: ${coverage.numericPercent.toFixed(1)}%`}
                    />
                    <div
                        className="bg-purple-500 h-full transition-all duration-500"
                        style={{ width: `${coverage.symbolicPercent}%` }}
                        title={`Symbolic: ${coverage.symbolicPercent.toFixed(1)}%`}
                    />
                    <div
                        className="bg-slate-400 h-full transition-all duration-500"
                        style={{ width: `${coverage.indirectPercent}%` }}
                        title={`Indirect: ${coverage.indirectPercent.toFixed(1)}%`}
                    />
                    <div
                        className="bg-red-400 h-full transition-all duration-500"
                        style={{ width: `${coverage.unsupportedPercent}%` }}
                        title={`Unsupported: ${coverage.unsupportedPercent.toFixed(1)}%`}
                    />
                </div>
            </div>

            {/* Coverage Legend */}
            <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-green-500"></div>
                    <span className="text-slate-600">Numeric {coverage.numericPercent.toFixed(1)}%</span>
                </div>
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-purple-500"></div>
                    <span className="text-slate-600">Symbolic {coverage.symbolicPercent.toFixed(1)}%</span>
                </div>
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-slate-400"></div>
                    <span className="text-slate-600">Indirect {coverage.indirectPercent.toFixed(1)}%</span>
                </div>
                <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full bg-red-400"></div>
                    <span className="text-slate-600">Unsupported {coverage.unsupportedPercent.toFixed(1)}%</span>
                </div>
            </div>
        </div>
    )
}

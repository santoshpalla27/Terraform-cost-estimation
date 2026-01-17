import type { DiffResult, ResourceChange } from '../../types'

interface Props {
    diff: DiffResult
}

export function DiffSummary({ diff }: Props) {
    const deltaColor = diff.delta >= 0 ? 'text-red-600' : 'text-green-600'
    const deltaSign = diff.delta >= 0 ? '+' : ''

    return (
        <div className="bg-white rounded-xl shadow-sm border border-slate-200 overflow-hidden">
            {/* Header */}
            <div className="p-6 border-b border-slate-200 bg-gradient-to-r from-slate-50 to-white">
                <div className="flex items-center justify-between">
                    <h2 className="text-xl font-semibold text-slate-900">Cost Diff</h2>
                    <div className={`text-2xl font-bold ${deltaColor}`}>
                        {deltaSign}${Math.abs(diff.delta).toFixed(2)}
                        <span className="text-sm ml-1">({deltaSign}{diff.deltaPercent.toFixed(1)}%)</span>
                    </div>
                </div>
            </div>

            {/* Before/After */}
            <div className="grid grid-cols-2 divide-x divide-slate-200">
                <div className="p-6 text-center">
                    <div className="text-sm text-slate-500 uppercase tracking-wide">Before</div>
                    <div className="text-3xl font-bold text-slate-900 mt-2">
                        ${diff.oldCost.toFixed(2)}
                    </div>
                    <div className="text-sm text-slate-500 mt-1">
                        {(diff.oldConfidence * 100).toFixed(0)}% confidence
                    </div>
                </div>
                <div className="p-6 text-center">
                    <div className="text-sm text-slate-500 uppercase tracking-wide">After</div>
                    <div className="text-3xl font-bold text-slate-900 mt-2">
                        ${diff.newCost.toFixed(2)}
                    </div>
                    <div className="text-sm text-slate-500 mt-1">
                        {(diff.newConfidence * 100).toFixed(0)}% confidence
                    </div>
                </div>
            </div>

            {/* Summary stats */}
            <div className="grid grid-cols-3 divide-x divide-slate-200 border-t border-slate-200">
                <div className="p-4 text-center">
                    <div className="text-2xl font-bold text-green-600">{diff.summary.createdCount}</div>
                    <div className="text-xs text-slate-500">Created</div>
                </div>
                <div className="p-4 text-center">
                    <div className="text-2xl font-bold text-blue-600">{diff.summary.updatedCount}</div>
                    <div className="text-xs text-slate-500">Updated</div>
                </div>
                <div className="p-4 text-center">
                    <div className="text-2xl font-bold text-red-600">{diff.summary.destroyedCount}</div>
                    <div className="text-xs text-slate-500">Destroyed</div>
                </div>
            </div>

            {/* Top increases */}
            {diff.summary.topIncreases.length > 0 && (
                <div className="p-4 border-t border-slate-200">
                    <h3 className="text-sm font-medium text-slate-700 mb-3">Top Cost Increases</h3>
                    <div className="space-y-2">
                        {diff.summary.topIncreases.slice(0, 5).map((change) => (
                            <ResourceChangeRow key={change.address} change={change} />
                        ))}
                    </div>
                </div>
            )}

            {/* Symbolic warnings */}
            {diff.summary.symbolicChanges.length > 0 && (
                <div className="p-4 border-t border-slate-200 bg-yellow-50">
                    <h3 className="text-sm font-medium text-yellow-800 mb-2">⚠ Symbolic Costs</h3>
                    <ul className="text-sm text-yellow-700 space-y-1">
                        {diff.summary.symbolicChanges.map((msg, i) => (
                            <li key={i}>• {msg}</li>
                        ))}
                    </ul>
                </div>
            )}
        </div>
    )
}

function ResourceChangeRow({ change }: { change: ResourceChange }) {
    const deltaColor = change.delta >= 0 ? 'text-red-600' : 'text-green-600'
    const changeTypeColors = {
        create: 'bg-green-100 text-green-800',
        destroy: 'bg-red-100 text-red-800',
        update: 'bg-blue-100 text-blue-800',
        no_change: 'bg-slate-100 text-slate-800',
    }

    return (
        <div className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
            <div className="flex items-center gap-3">
                <span className={`text-xs px-2 py-0.5 rounded ${changeTypeColors[change.changeType]}`}>
                    {change.changeType}
                </span>
                <span className="font-mono text-sm text-slate-900">{change.address}</span>
            </div>
            <div className={`font-semibold ${deltaColor}`}>
                {change.delta >= 0 ? '+' : ''}${change.delta.toFixed(2)}
            </div>
        </div>
    )
}

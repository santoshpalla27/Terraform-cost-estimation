import type { CostLineage } from '../../types'

interface Props {
    lineage: CostLineage
    onClose: () => void
}

export function CostLineagePopover({ lineage, onClose }: Props) {
    return (
        <div className="absolute z-50 bg-white rounded-lg shadow-xl border border-slate-200 p-4 w-80">
            <div className="flex items-center justify-between mb-3">
                <h4 className="font-semibold text-slate-900">Cost Lineage</h4>
                <button
                    onClick={onClose}
                    className="text-slate-400 hover:text-slate-600"
                >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            <div className="space-y-3 text-sm">
                {/* Rate Key */}
                <div>
                    <div className="text-xs font-medium text-slate-500 uppercase mb-1">Rate Key</div>
                    <div className="bg-slate-50 rounded p-2 font-mono text-xs">
                        <div><span className="text-slate-500">Cloud:</span> {lineage.rateKey.cloud}</div>
                        <div><span className="text-slate-500">Service:</span> {lineage.rateKey.service}</div>
                        <div><span className="text-slate-500">Family:</span> {lineage.rateKey.productFamily}</div>
                        <div><span className="text-slate-500">Region:</span> {lineage.rateKey.region}</div>
                    </div>
                </div>

                {/* Attributes */}
                <div>
                    <div className="text-xs font-medium text-slate-500 uppercase mb-1">Attributes</div>
                    <div className="bg-slate-50 rounded p-2 font-mono text-xs space-y-1">
                        {Object.entries(lineage.rateKey.attributes).map(([k, v]) => (
                            <div key={k}>
                                <span className="text-slate-500">{k}:</span> {v}
                            </div>
                        ))}
                    </div>
                </div>

                {/* Snapshot */}
                <div>
                    <div className="text-xs font-medium text-slate-500 uppercase mb-1">Pricing Snapshot</div>
                    <div className="text-xs">
                        <div><span className="text-slate-500">ID:</span> <span className="font-mono">{lineage.snapshotId}</span></div>
                        <div><span className="text-slate-500">Source:</span> {lineage.source}</div>
                        <div><span className="text-slate-500">Resolved:</span> {lineage.resolvedAt}</div>
                    </div>
                </div>
            </div>

            <div className="mt-4 pt-3 border-t border-slate-200 text-xs text-slate-500">
                This cost is deterministic and reproducible using the snapshot ID above.
            </div>
        </div>
    )
}

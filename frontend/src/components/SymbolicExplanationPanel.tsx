import type { ResourceCost } from '../types'

interface Props {
    resource: ResourceCost
    reasons: string[]
    onClose: () => void
}

export function SymbolicExplanationPanel({ resource, reasons, onClose }: Props) {
    return (
        <div className="bg-yellow-50 border border-yellow-200 rounded-xl p-6">
            <div className="flex items-start justify-between mb-4">
                <div className="flex items-center gap-3">
                    <div className="w-10 h-10 bg-yellow-100 rounded-full flex items-center justify-center text-xl">
                        ⚠️
                    </div>
                    <div>
                        <h3 className="text-lg font-semibold text-yellow-900">
                            Cost Cannot Be Fully Estimated
                        </h3>
                        <p className="text-sm text-yellow-700 font-mono">{resource.address}</p>
                    </div>
                </div>
                <button
                    onClick={onClose}
                    className="text-yellow-700 hover:text-yellow-900"
                >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            {/* Reasons */}
            <div className="mb-6">
                <h4 className="text-sm font-medium text-yellow-900 mb-2">Reason</h4>
                <ul className="space-y-2">
                    {reasons.map((reason, i) => (
                        <li key={i} className="flex items-start gap-2 text-sm text-yellow-800">
                            <span className="text-yellow-600 mt-0.5">•</span>
                            {reason}
                        </li>
                    ))}
                </ul>
            </div>

            {/* Impact */}
            <div className="mb-6 p-4 bg-yellow-100 rounded-lg">
                <h4 className="text-sm font-medium text-yellow-900 mb-2">Impact</h4>
                <ul className="space-y-1 text-sm text-yellow-800">
                    <li>• Cost shown as <span className="font-medium">symbolic</span> (may differ from actual)</li>
                    <li>• Confidence reduced by <span className="font-mono">{((1 - resource.confidence) * 100).toFixed(0)}%</span></li>
                    <li>• Coverage classified as <span className="font-medium">symbolic</span> in reports</li>
                </ul>
            </div>

            {/* Action */}
            <div className="p-4 bg-white rounded-lg border border-yellow-200">
                <h4 className="text-sm font-medium text-slate-900 mb-3">Action Required</h4>
                <p className="text-sm text-slate-600 mb-3">
                    Provide a usage override to convert this to a numeric cost:
                </p>
                <div className="bg-slate-900 text-green-400 rounded p-3 font-mono text-xs overflow-x-auto">
                    <div className="text-slate-500"># In your usage file:</div>
                    <div>{resource.type}:</div>
                    <div className="ml-4">{resource.address.split('.').pop()}:</div>
                    <div className="ml-8">monthly_requests: 1000000</div>
                    <div className="ml-8">avg_duration_ms: 200</div>
                </div>
            </div>

            {/* Current estimate */}
            <div className="mt-4 pt-4 border-t border-yellow-200 flex items-center justify-between text-sm">
                <span className="text-yellow-700">Current symbolic estimate:</span>
                <span className="font-semibold text-yellow-900">
                    ${resource.cost.toFixed(2)}/month
                </span>
            </div>
        </div>
    )
}

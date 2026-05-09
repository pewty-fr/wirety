import { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';
import type { Peer } from '../types';
import { useACL } from '../hooks/useQueries';

interface NetworkTopologyProps {
  peer: Peer;
  allPeers: Peer[];
}

export default function NetworkTopology({ peer, allPeers }: NetworkTopologyProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  // Use React Query to load ACL
  const { data: acl } = useACL(peer.network_id || '', !!peer.network_id);

  useEffect(() => {
    // Generate Mermaid diagram definition
    const generateMermaidDiagram = (): string => {
      const lines: string[] = ['graph TB'];
      
      // Add styling
      lines.push('  classDef jump fill:#e9d5ff,stroke:#a855f7,stroke-width:3px');
      lines.push('  classDef isolated fill:#fef3c7,stroke:#f59e0b,stroke-width:2px');
      lines.push('  classDef regular fill:#d1fae5,stroke:#10b981,stroke-width:2px');
      lines.push('  classDef current fill:#dbeafe,stroke:#3b82f6,stroke-width:4px');
      lines.push('  classDef blocked fill:#fca5a5,stroke:#dc2626,stroke-width:3px,stroke-dasharray:5 5');
      lines.push('  classDef connected stroke:#22c55e,stroke-width:3px');
      lines.push('  classDef disconnected stroke:#ef4444,stroke-width:2px,stroke-dasharray:5 5');
      lines.push('');

      // Determine accessibility rules
      const canAccess = (from: Peer, to: Peer): boolean => {
        // Can't connect to self
        if (from.id === to.id) return false;

        // Check ACL - if either peer is blocked, no access
        if (acl?.enabled && acl.blocked_peers) {
          if (acl.blocked_peers[from.id] || acl.blocked_peers[to.id]) {
            return false;
          }
        }

        // Jump peers can access everything; everyone can access jump peers.
        // Regular-to-regular access is gated by groups/policies, which are not
        // resolved here — visually we just show full connectivity.
        if (from.is_jump || to.is_jump) return true;
        return true;
      };

      // Filter to only accessible peers
      const accessiblePeers = allPeers.filter(p => canAccess(peer, p) || p.id === peer.id);

      // Add current peer (highlighted)
      const currentLabel = `${peer.name}<br/>${peer.address}`;
      lines.push(`  PEER_${peer.id}["${currentLabel}"]:::current`);
      lines.push('');

      // Add accessible peers only
      accessiblePeers.forEach(p => {
        if (p.id === peer.id) return; // Skip current peer
        const isBlocked = acl?.enabled && acl.blocked_peers && acl.blocked_peers[p.id];
        
        const label = `${p.name}<br/>${p.address}`;
        const className = isBlocked ? 'blocked' : (p.is_jump ? 'jump' : 'regular');
        lines.push(`  PEER_${p.id}["${label}"]:::${className}`);
      });
      lines.push('');

      // Add connections from current peer to accessible peers
      accessiblePeers.forEach(targetPeer => {
        if (targetPeer.id === peer.id) return;

        // Edge labeling — connection state inferred from heartbeat session.
        const isConnected = targetPeer.is_jump && !!targetPeer.session_status?.current_session?.reported_endpoint;
        const lineStyle = isConnected ? '===' : '-.-';
        const label = targetPeer.is_jump
          ? (isConnected ? 'Connected' : 'Down')
          : 'Can Access';
        lines.push(`  PEER_${peer.id} ${lineStyle}>|${label}| PEER_${targetPeer.id}`);
      });

      return lines.join('\n');
    };
    const renderDiagram = async () => {
      if (!containerRef.current) return;

      try {
        setError(null);
        const diagram = generateMermaidDiagram();
        
        // Clear previous content
        containerRef.current.innerHTML = '';

        // Using Mermaid's native rendering
        mermaid.initialize({
          startOnLoad: false,
          theme: 'default',
          flowchart: {
            useMaxWidth: true,
            htmlLabels: true,
            curve: 'basis',
          },
        });

        const { svg } = await mermaid.render(`mermaid-${peer.id}-${Date.now()}`, diagram);
        containerRef.current.innerHTML = svg;
      } catch (err) {
        const error = err as { message?: string };
        console.error('Failed to render network topology:', err);
        setError(error.message || 'Failed to render topology');
      }
    };

    renderDiagram();
  }, [peer, allPeers, acl]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-gray-700">Network Topology</h4>
        <div className="flex gap-2 text-xs">
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 bg-primary-200 border-2 border-primary-600 rounded dark:bg-primary-300 dark:border-primary-500"></div>
            <span className="text-gray-600">Current</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 bg-purple-200 border-2 border-purple-600 rounded"></div>
            <span className="text-gray-600">Jump</span>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 bg-green-200 border-2 border-green-600 rounded"></div>
            <span className="text-gray-600">Regular</span>
                    <div className="flex items-center gap-1">
                      <div className="w-3 h-3 bg-red-200 border-2 border-red-600 rounded border-dashed"></div>
                      <span className="text-gray-600">Blocked</span>
                    </div>
          </div>
          <div className="flex items-center gap-1">
            <div className="w-3 h-3 bg-yellow-200 border-2 border-yellow-600 rounded"></div>
            <span className="text-gray-600">Isolated</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">
          {error}
        </div>
      )}

      <div 
        ref={containerRef}
        className="bg-white border border-gray-200 dark:border-gray-700 rounded-lg p-4 overflow-auto"
        style={{ minHeight: '300px' }}
      />

      <div className="text-xs text-gray-500 space-y-1">
        <p>• <strong>Solid lines (===)</strong>: Active connection to jump server</p>
        <p>• <strong>Dashed lines (- - -)</strong>: Inactive jump connection</p>
        <p>• Only direct edges from current peer (no transit via jump)</p>
      </div>
    </div>
  );
}

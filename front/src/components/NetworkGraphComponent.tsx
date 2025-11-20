import React, { useState } from 'react';
import { View, StyleSheet, Dimensions, PanResponder } from 'react-native';
import { Text, Card } from 'react-native-paper';
import Svg, { 
  Circle, 
  Line, 
  Text as SvgText, 
  G,
  Defs,
  Marker,
  Path
} from 'react-native-svg';

interface GraphNode {
  id: string;
  name: string;
  type: 'current' | 'jump' | 'regular' | 'internet' | 'network';
  address?: string;
  endpoint?: string;
  isolated?: boolean;
  fullEncapsulation?: boolean;
  is_jump?: boolean;
  jump_nat_interface?: string;
  is_isolated?: boolean;
  full_encapsulation?: boolean;
}

interface GraphEdge {
  from: string;
  to: string;
  type: 'direct' | 'tunnel' | 'blocked' | 'internet';
  label?: string;
}

interface NetworkGraphComponentProps {
  nodes: GraphNode[];
  edges: GraphEdge[];
  currentPeerId: string;
}

interface NodePosition {
  id: string;
  x: number;
  y: number;
  radius: number;
  color: string;
  node: GraphNode;
}

export const NetworkGraphComponent: React.FC<NetworkGraphComponentProps> = ({
  nodes,
  edges,
  currentPeerId
}) => {
  const screenWidth = Dimensions.get('window').width - 32;
  const diagramHeight = 480;
  const [zoom, setZoom] = useState(1);
  const [panX, setPanX] = useState(0);
  const [panY, setPanY] = useState(0);
  
  // Calculate positions for all nodes
  const calculateNodePositions = (): NodePosition[] => {
    const positions: NodePosition[] = [];
    const centerX = screenWidth / 2;
    const centerY = diagramHeight / 2;
    
    nodes.forEach((node, index) => {
      let x, y;
      const radius = node.type === 'current' ? 30 : node.type === 'jump' ? 25 : 20;
      const color = getNodeColor(node);
      
      // Radial layered layout: current peer center, jumps inner ring, regular outer ring, internet top.
      if (node.id === currentPeerId) {
        x = centerX; y = centerY;
      } else if (node.type === 'internet') {
        x = centerX; y = centerY - 170;
      } else {
        const jumpNodes = nodes.filter(n => n.type === 'jump');
        const regularNodes = nodes.filter(n => n.type === 'regular');
        if (node.type === 'jump') {
          const jumpIndex = jumpNodes.findIndex(n => n.id === node.id);
          const angle = (jumpIndex * 2 * Math.PI) / Math.max(jumpNodes.length, 1);
          x = centerX + Math.cos(angle) * 110;
          y = centerY + Math.sin(angle) * 110;
        } else {
          const regularIndex = regularNodes.findIndex(n => n.id === node.id);
          const angle = (regularIndex * 2 * Math.PI) / Math.max(regularNodes.length, 1);
          x = centerX + Math.cos(angle) * 190;
          y = centerY + Math.sin(angle) * 190;
        }
      }
      
      positions.push({
        id: node.id,
        x,
        y,
        radius,
        color,
        node
      });
    });
    
    return positions;
  };
  
  const getNodeColor = (node: GraphNode): string => {
    if (node.id === currentPeerId) return '#2196F3';
    if (node.type === 'jump') return '#FF9800';
    if (node.type === 'internet') return '#9C27B0';
    if (node.isolated || node.is_isolated) return '#F44336';
    return '#4CAF50';
  };
  
  const getEdgeColor = (edgeType: string): string => {
    switch (edgeType) {
      case 'direct': return '#4CAF50';
      case 'tunnel': return '#2196F3';
      case 'blocked': return '#F44336';
      case 'internet': return '#9C27B0';
      default: return '#757575';
    }
  };
  
  const getStrokeDashArray = (edgeType: string): string => {
    switch (edgeType) {
      case 'blocked': return '6,3';
      case 'tunnel': return '10,4';
      default: return '';
    }
  };
  
  const positions = calculateNodePositions();
  
  const getPosition = (nodeId: string) => {
    return positions.find(p => p.id === nodeId);
  };
  
  // Create pan responder for zoom and pan functionality
  const panResponder = PanResponder.create({
    onMoveShouldSetPanResponder: () => true,
    onPanResponderMove: (evt, gestureState) => {
      setPanX(panX + gestureState.dx * 0.5);
      setPanY(panY + gestureState.dy * 0.5);
    },
  });

  if (nodes.length === 0) {
    return (
      <View style={styles.emptyContainer}>
        <Text>No network data available</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Card style={styles.diagramCard}>
        <Card.Content>
          <Text style={styles.title}>Interactive Network Topology</Text>
          
          {/* Zoom controls */}
          <View style={styles.controls}>
            <Text 
              style={styles.controlButton} 
              onPress={() => setZoom(Math.min(zoom * 1.2, 3))}
            >
              🔍+
            </Text>
            <Text style={styles.zoomLevel}>Zoom: {(zoom * 100).toFixed(0)}%</Text>
            <Text 
              style={styles.controlButton} 
              onPress={() => setZoom(Math.max(zoom / 1.2, 0.3))}
            >
              🔍-
            </Text>
            <Text 
              style={styles.controlButton} 
              onPress={() => {
                setZoom(1);
                setPanX(0);
                setPanY(0);
              }}
            >
              🏠
            </Text>
          </View>
          
          <View 
            style={[styles.diagramContainer, { height: diagramHeight }]}
            {...panResponder.panHandlers}
          >
            <Svg 
              width={screenWidth} 
              height={diagramHeight}
              viewBox={`${-panX/zoom} ${-panY/zoom} ${screenWidth/zoom} ${diagramHeight/zoom}`}
            >
              <Defs>
                <Marker id="arrowhead" markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
                  <Path d="M0,0 L0,6 L9,3 z" fill="#666" />
                </Marker>
              </Defs>
              
              <G scale={zoom}>
                {/* Render edges first so they appear behind nodes */}
                {edges.map((edge, index) => {
                  const fromPos = getPosition(edge.from);
                  const toPos = getPosition(edge.to);
                  if (!fromPos || !toPos) return null;
                  const dx = toPos.x - fromPos.x;
                  const dy = toPos.y - fromPos.y;
                  const distance = Math.sqrt(dx * dx + dy * dy);
                  const unitX = dx / distance;
                  const unitY = dy / distance;
                  const startX = fromPos.x + unitX * fromPos.radius;
                  const startY = fromPos.y + unitY * fromPos.radius;
                  const endX = toPos.x - unitX * toPos.radius;
                  const endY = toPos.y - unitY * toPos.radius;
                  // Curved path control point (perpendicular offset)
                  const midX = (startX + endX) / 2;
                  const midY = (startY + endY) / 2;
                  const perpX = -unitY;
                  const perpY = unitX;
                  const curveStrength = Math.min(40, distance / 6);
                  const cpx = midX + perpX * curveStrength;
                  const cpy = midY + perpY * curveStrength;
                  const pathData = `M ${startX} ${startY} Q ${cpx} ${cpy} ${endX} ${endY}`;
                  return (
                    <G key={`edge-${index}`}>
                      <Path
                        d={pathData}
                        stroke={getEdgeColor(edge.type)}
                        strokeWidth={2}
                        fill="none"
                        strokeDasharray={getStrokeDashArray(edge.type)}
                        markerEnd="url(#arrowhead)"
                      />
                      {edge.label && (
                        <SvgText
                          x={cpx}
                          y={cpy - 6}
                          textAnchor="middle"
                          fontSize="10"
                          fill="#555"
                          fontWeight="500"
                        >
                          {edge.label}
                        </SvgText>
                      )}
                    </G>
                  );
                })}
                
                {/* Render nodes on top of edges */}
                {positions.map((pos) => {
                  const isIsolated = pos.node.isolated || pos.node.is_isolated;
                  const isFullEncap = pos.node.fullEncapsulation || pos.node.full_encapsulation;
                  
                  return (
                    <G key={pos.id}>
                      {/* Node shadow for depth */}
                      <Circle
                        cx={pos.x + 2}
                        cy={pos.y + 2}
                        r={pos.radius}
                        fill="rgba(0,0,0,0.2)"
                      />
                      
                      {/* Main node circle */}
                      <Circle
                        cx={pos.x}
                        cy={pos.y}
                        r={pos.radius}
                        fill={pos.color}
                        stroke="#FFFFFF"
                        strokeWidth="2"
                      />
                      
                      {/* Node name */}
                      <SvgText
                        x={pos.x}
                        y={pos.y}
                        textAnchor="middle"
                        fontSize="11"
                        fill="#FFFFFF"
                        fontWeight="600"
                      >
                        {pos.node.name.length > 10 ? pos.node.name.substring(0, 10) + '…' : pos.node.name}
                      </SvgText>
                      
                      {/* Address below node */}
                      {pos.node.address && (
                        <SvgText
                          x={pos.x}
                          y={pos.y + pos.radius + 14}
                          textAnchor="middle"
                          fontSize="9"
                          fill="#444"
                        >
                          {pos.node.address}
                        </SvgText>
                      )}
                      
                      {/* Status indicators */}
                      {isIsolated && (
                        <SvgText
                          x={pos.x}
                          y={pos.y + pos.radius + 26}
                          textAnchor="middle"
                          fontSize="8"
                          fill="#F44336"
                          fontWeight="bold"
                        >
                          ISOLATED
                        </SvgText>
                      )}
                      {isFullEncap && (
                        <SvgText
                          x={pos.x}
                          y={pos.y + pos.radius + (isIsolated ? 38 : 26)}
                          textAnchor="middle"
                          fontSize="8"
                          fill="#9C27B0"
                          fontWeight="bold"
                        >
                          FULL TUNNEL
                        </SvgText>
                      )}
                    </G>
                  );
                })}
              </G>
            </Svg>
          </View>
        </Card.Content>
      </Card>
      
      {/* Legend */}
      <Card style={styles.legendCard}>
        <Card.Content>
          <Text style={styles.legendTitle}>Legend</Text>
          <View style={styles.legendGrid}>
            <View style={styles.legendSection}>
              <Text style={styles.legendSubtitle}>Node Types:</Text>
              <View style={styles.legendRow}>
                <View style={[styles.legendColor, { backgroundColor: '#2196F3' }]} />
                <Text style={styles.legendText}>Current Peer</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendColor, { backgroundColor: '#FF9800' }]} />
                <Text style={styles.legendText}>Jump Server</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendColor, { backgroundColor: '#4CAF50' }]} />
                <Text style={styles.legendText}>Regular Peer</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendColor, { backgroundColor: '#F44336' }]} />
                <Text style={styles.legendText}>Isolated Peer</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendColor, { backgroundColor: '#9C27B0' }]} />
                <Text style={styles.legendText}>Internet</Text>
              </View>
            </View>
            
            <View style={styles.legendSection}>
              <Text style={styles.legendSubtitle}>Connections:</Text>
              <View style={styles.legendRow}>
                <View style={[styles.legendLine, { backgroundColor: '#4CAF50' }]} />
                <Text style={styles.legendText}>Direct</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendLine, { backgroundColor: '#2196F3', opacity: 0.7 }]} />
                <Text style={styles.legendText}>Tunnel</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendLine, { backgroundColor: '#F44336', opacity: 0.7 }]} />
                <Text style={styles.legendText}>Blocked</Text>
              </View>
              <View style={styles.legendRow}>
                <View style={[styles.legendLine, { backgroundColor: '#9C27B0' }]} />
                <Text style={styles.legendText}>Internet</Text>
              </View>
            </View>
          </View>
        </Card.Content>
      </Card>

      <Card style={styles.instructionsCard}>
        <Card.Content>
          <Text style={styles.instructionsTitle}>Controls:</Text>
          <Text style={styles.instructionsText}>
            • Drag the diagram to pan around{'\n'}
            • Use +/- buttons to zoom in/out{'\n'}
            • Use 🏠 button to reset view{'\n'}
            • Node colors indicate peer types and status
          </Text>
        </Card.Content>
      </Card>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    padding: 16,
  },
  emptyContainer: {
    height: 200,
    justifyContent: 'center',
    alignItems: 'center',
  },
  diagramCard: {
    marginBottom: 16,
  },
  title: {
    fontSize: 18,
    fontWeight: 'bold',
    marginBottom: 12,
    textAlign: 'center',
  },
  controls: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 16,
    gap: 12,
  },
  controlButton: {
    backgroundColor: '#E3F2FD',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 6,
    fontSize: 16,
    fontWeight: 'bold',
    color: '#1976D2',
    textAlign: 'center',
  },
  zoomLevel: {
    fontSize: 14,
    color: '#666',
    minWidth: 80,
    textAlign: 'center',
  },
  diagramContainer: {
    width: '100%',
    borderWidth: 1,
    borderColor: '#E0E0E0',
    borderRadius: 8,
    overflow: 'hidden',
    backgroundColor: '#FAFAFA',
  },
  legendCard: {
    marginBottom: 16,
  },
  legendTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 12,
    textAlign: 'center',
  },
  legendGrid: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 16,
  },
  legendSection: {
    flex: 1,
  },
  legendSubtitle: {
    fontSize: 14,
    fontWeight: 'bold',
    marginBottom: 8,
    color: '#666',
  },
  legendRow: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 6,
  },
  legendColor: {
    width: 12,
    height: 12,
    borderRadius: 6,
    marginRight: 8,
  },
  legendLine: {
    width: 20,
    height: 3,
    marginRight: 8,
    borderRadius: 1,
  },
  legendText: {
    fontSize: 12,
    flex: 1,
  },
  instructionsCard: {
    marginBottom: 16,
  },
  instructionsTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  instructionsText: {
    fontSize: 14,
    lineHeight: 20,
    color: '#666',
  },
});

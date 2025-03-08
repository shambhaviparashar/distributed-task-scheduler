import React, { useEffect, useRef } from 'react';
import PropTypes from 'prop-types';
import { Box, Paper, Typography, CircularProgress } from '@mui/material';
import * as d3 from 'd3';
import dagreD3 from 'dagre-d3';

const STATUS_COLORS = {
  pending: '#9e9e9e',
  running: '#2196f3',
  completed: '#4caf50',
  failed: '#f44336',
  cancelled: '#ff9800',
  skipped: '#9c27b0',
  default: '#9e9e9e'
};

const TaskGraph = ({ tasks, edges, loading, error, width = 800, height = 600, onNodeClick }) => {
  const svgRef = useRef(null);
  
  useEffect(() => {
    if (loading || error || !tasks || !edges || tasks.length === 0) {
      return;
    }
    
    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();
    
    // Define the graph
    const g = new dagreD3.graphlib.Graph({ multigraph: true })
      .setGraph({
        rankdir: 'TB',
        marginx: 20,
        marginy: 20,
        nodeSep: 50,
        edgeSep: 100,
        ranksep: 75
      })
      .setDefaultEdgeLabel(() => ({}));
    
    // Add nodes to the graph
    tasks.forEach(task => {
      const color = STATUS_COLORS[task.status] || STATUS_COLORS.default;
      g.setNode(task.id, {
        label: task.name || task.id,
        class: `task-status-${task.status}`,
        labelStyle: 'font-size: 14px; font-weight: bold;',
        style: `fill: ${color}; stroke: #333; stroke-width: 1.5px;`,
        rx: 5,
        ry: 5,
        task: task, // Store the whole task object
      });
    });
    
    // Add edges to the graph
    edges.forEach(edge => {
      g.setEdge(edge.source, edge.target, {
        arrowheadClass: 'arrowhead',
        curve: d3.curveBasis,
        style: edge.conditional 
          ? 'stroke: blue; stroke-width: 1.5px; stroke-dasharray: 5, 5;' 
          : 'stroke: #333; stroke-width: 1.5px;'
      });
    });
    
    // Create the renderer
    const render = new dagreD3.render();
    
    // Set up the SVG group
    const svgGroup = svg.append('g');
    
    // Run the renderer
    render(svgGroup, g);
    
    // Add zoom behavior
    const zoom = d3.zoom().on('zoom', (event) => {
      svgGroup.attr('transform', event.transform);
    });
    svg.call(zoom);
    
    // Center the graph
    const initialScale = 0.8;
    const svgWidth = svg.node().getBoundingClientRect().width;
    const svgHeight = svg.node().getBoundingClientRect().height;
    const graphWidth = g.graph().width;
    const graphHeight = g.graph().height;
    const xCenterOffset = (svgWidth - graphWidth * initialScale) / 2;
    const yCenterOffset = (svgHeight - graphHeight * initialScale) / 2;
    
    svg.call(zoom.transform, d3.zoomIdentity
      .translate(xCenterOffset, yCenterOffset)
      .scale(initialScale));
    
    // Add tooltips
    svgGroup.selectAll('g.node')
      .append('title')
      .text(d => {
        const task = g.node(d).task;
        return `ID: ${task.id}\nStatus: ${task.status}\nType: ${task.type || 'N/A'}`;
      });
    
    // Add click handlers
    svgGroup.selectAll('g.node')
      .on('click', (event, id) => {
        if (onNodeClick) {
          const task = g.node(id).task;
          onNodeClick(task);
        }
      })
      .on('mouseover', function() {
        d3.select(this).style('cursor', 'pointer');
      })
      .on('mouseout', function() {
        d3.select(this).style('cursor', 'default');
      });
    
  }, [tasks, edges, loading, error, width, height, onNodeClick]);
  
  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: height }}>
        <CircularProgress />
      </Box>
    );
  }
  
  if (error) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: height }}>
        <Typography color="error">Error loading graph: {error.message}</Typography>
      </Box>
    );
  }
  
  if (!tasks || tasks.length === 0) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: height }}>
        <Typography>No tasks to display.</Typography>
      </Box>
    );
  }
  
  return (
    <Paper sx={{ width: '100%', height: height, overflow: 'hidden', borderRadius: 2 }}>
      <svg ref={svgRef} width="100%" height="100%">
        <defs>
          <marker
            id="arrowhead"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerUnits="strokeWidth"
            markerWidth="8"
            markerHeight="6"
            orient="auto"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#333" />
          </marker>
        </defs>
      </svg>
    </Paper>
  );
};

TaskGraph.propTypes = {
  tasks: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string,
      status: PropTypes.string,
      type: PropTypes.string
    })
  ),
  edges: PropTypes.arrayOf(
    PropTypes.shape({
      source: PropTypes.string.isRequired,
      target: PropTypes.string.isRequired,
      conditional: PropTypes.bool
    })
  ),
  loading: PropTypes.bool,
  error: PropTypes.object,
  width: PropTypes.number,
  height: PropTypes.number,
  onNodeClick: PropTypes.func
};

export default TaskGraph; 
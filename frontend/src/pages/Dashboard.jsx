import React from 'react';
import { useQuery } from 'react-query';
import { 
  Box, 
  Card, 
  CardContent, 
  Grid, 
  Typography, 
  CircularProgress, 
  Divider, 
  Button,
  Paper,
  Stack,
  IconButton,
  Tooltip,
  useTheme
} from '@mui/material';
import { 
  Refresh as RefreshIcon,
  Queue as QueueIcon,
  AccessTime as TimeIcon,
  CheckCircle as SuccessIcon,
  Error as ErrorIcon,
  Warning as WarningIcon,
  Schedule as ScheduleIcon,
  Memory as MemoryIcon,
  Dns as DnsIcon,
  Storage as StorageIcon
} from '@mui/icons-material';
import { 
  Chart as ChartJS, 
  CategoryScale, 
  LinearScale, 
  PointElement, 
  LineElement, 
  BarElement, 
  Title, 
  Tooltip as ChartTooltip, 
  Legend, 
  ArcElement 
} from 'chart.js';
import { Line, Bar, Pie } from 'react-chartjs-2';
import { Link } from 'react-router-dom';
import TaskStatusCard from '../components/TaskStatusCard';
import MetricCard from '../components/MetricCard';
import { fetchSystemStatus, fetchSystemMetrics } from '../services/api';

// Register ChartJS components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Title,
  ChartTooltip,
  Legend,
  ArcElement
);

const Dashboard = () => {
  const theme = useTheme();
  
  // Fetch system status
  const { 
    data: statusData, 
    isLoading: statusLoading, 
    error: statusError,
    refetch: refetchStatus
  } = useQuery('systemStatus', fetchSystemStatus, {
    refetchInterval: 30000, // Refetch every 30 seconds
  });
  
  // Fetch system metrics
  const { 
    data: metricsData, 
    isLoading: metricsLoading, 
    error: metricsError,
    refetch: refetchMetrics
  } = useQuery('systemMetrics', () => fetchSystemMetrics('hour'), {
    refetchInterval: 60000, // Refetch every minute
  });
  
  const handleRefresh = () => {
    refetchStatus();
    refetchMetrics();
  };
  
  // Calculate component status icons
  const getComponentStatusIcon = (status) => {
    switch (status) {
      case 'healthy':
        return <SuccessIcon fontSize="small" color="success" />;
      case 'degraded':
        return <WarningIcon fontSize="small" color="warning" />;
      case 'unhealthy':
        return <ErrorIcon fontSize="small" color="error" />;
      default:
        return <WarningIcon fontSize="small" color="disabled" />;
    }
  };
  
  // Prepare data for task status distribution chart
  const taskStatusData = statusData ? {
    labels: ['Pending', 'Running', 'Completed', 'Failed', 'Cancelled'],
    datasets: [
      {
        label: 'Tasks',
        data: [
          statusData.task_stats.pending || 0,
          statusData.task_stats.running || 0,
          statusData.task_stats.completed_today || 0,
          statusData.task_stats.failed_today || 0,
          statusData.task_stats.cancelled || 0,
        ],
        backgroundColor: [
          '#9e9e9e',
          '#2196f3',
          '#4caf50',
          '#f44336',
          '#ff9800',
        ],
        borderWidth: 1,
      },
    ],
  } : null;
  
  // Prepare data for execution time chart
  const executionTimeData = metricsData ? {
    labels: metricsData.average_execution_time.map(item => 
      new Date(item.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    ),
    datasets: [
      {
        label: 'Average Execution Time (seconds)',
        data: metricsData.average_execution_time.map(item => item.value),
        borderColor: theme.palette.primary.main,
        backgroundColor: 'rgba(33, 150, 243, 0.1)',
        fill: true,
        tension: 0.4,
      },
    ],
  } : null;
  
  // Prepare data for completion rate chart
  const completionRateData = metricsData ? {
    labels: metricsData.task_completion_rate.map(item => 
      new Date(item.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    ),
    datasets: [
      {
        label: 'Tasks Completed',
        data: metricsData.task_completion_rate.map(item => item.value),
        backgroundColor: theme.palette.success.main,
      },
    ],
  } : null;
  
  // Prepare data for error rate chart
  const errorRateData = metricsData ? {
    labels: metricsData.error_rate.map(item => 
      new Date(item.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    ),
    datasets: [
      {
        label: 'Error Rate (%)',
        data: metricsData.error_rate.map(item => item.value * 100), // Convert to percentage
        borderColor: theme.palette.error.main,
        backgroundColor: 'rgba(244, 67, 54, 0.1)',
        fill: true,
        tension: 0.4,
      },
    ],
  } : null;
  
  // Options for line charts
  const lineChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'top',
      },
    },
    scales: {
      y: {
        beginAtZero: true,
      },
    },
  };
  
  // Options for bar charts
  const barChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'top',
      },
    },
    scales: {
      y: {
        beginAtZero: true,
      },
    },
  };
  
  // Options for pie charts
  const pieChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        position: 'right',
      },
    },
  };
  
  // If both loading, show loading spinner
  if (statusLoading && metricsLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '80vh' }}>
        <CircularProgress />
      </Box>
    );
  }
  
  // If both have errors, show error message
  if (statusError && metricsError) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography color="error" variant="h6">
          Error loading dashboard data. Please try again later.
        </Typography>
        <Button startIcon={<RefreshIcon />} variant="contained" onClick={handleRefresh} sx={{ mt: 2 }}>
          Retry
        </Button>
      </Box>
    );
  }
  
  return (
    <Box sx={{ p: 3 }}>
      {/* Dashboard Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Typography variant="h4" component="h1">
          System Dashboard
        </Typography>
        <Button 
          startIcon={<RefreshIcon />} 
          variant="outlined" 
          onClick={handleRefresh}
          disabled={statusLoading || metricsLoading}
        >
          Refresh
        </Button>
      </Box>
      
      {/* Status Cards Row */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <TaskStatusCard 
            title="Pending Tasks"
            count={statusData?.task_stats.pending || 0}
            icon={<ScheduleIcon sx={{ fontSize: 40 }} color="disabled" />}
            color="#9e9e9e"
            to="/tasks?status=pending"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <TaskStatusCard 
            title="Running Tasks"
            count={statusData?.task_stats.running || 0}
            icon={<QueueIcon sx={{ fontSize: 40 }} color="primary" />}
            color="#2196f3"
            to="/tasks?status=running"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <TaskStatusCard 
            title="Completed Today"
            count={statusData?.task_stats.completed_today || 0}
            icon={<SuccessIcon sx={{ fontSize: 40 }} color="success" />}
            color="#4caf50"
            to="/tasks?status=completed"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <TaskStatusCard 
            title="Failed Today"
            count={statusData?.task_stats.failed_today || 0}
            icon={<ErrorIcon sx={{ fontSize: 40 }} color="error" />}
            color="#f44336"
            to="/tasks?status=failed"
          />
        </Grid>
      </Grid>
      
      {/* System Status Card */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            System Status
          </Typography>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Typography variant="subtitle1" gutterBottom>
                  Components
                </Typography>
                <Stack spacing={2}>
                  {statusData?.components ? (
                    Object.entries(statusData.components).map(([name, info]) => (
                      <Box key={name} sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                          {name === 'api_server' && <DnsIcon sx={{ mr: 1 }} />}
                          {name === 'scheduler' && <ScheduleIcon sx={{ mr: 1 }} />}
                          {name === 'workers' && <MemoryIcon sx={{ mr: 1 }} />}
                          {name === 'database' && <StorageIcon sx={{ mr: 1 }} />}
                          {name === 'redis' && <StorageIcon sx={{ mr: 1 }} />}
                          <Typography>
                            {name.replace('_', ' ').replace(/\b\w/g, l => l.toUpperCase())}
                            {info.instance_count && ` (${info.instance_count} instances)`}
                            {info.active_count && ` (${info.active_count} active)`}
                          </Typography>
                        </Box>
                        <Box sx={{ display: 'flex', alignItems: 'center' }}>
                          {getComponentStatusIcon(info.status)}
                          <Typography variant="body2" sx={{ ml: 1 }}>
                            {info.status}
                          </Typography>
                        </Box>
                      </Box>
                    ))
                  ) : (
                    <Typography color="text.secondary">No component data available</Typography>
                  )}
                </Stack>
              </Paper>
            </Grid>
            <Grid item xs={12} md={6}>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Typography variant="subtitle1" gutterBottom>
                  Queue Depths
                </Typography>
                {statusData?.queue_depths ? (
                  <Box sx={{ display: 'flex', height: 100 }}>
                    <Bar 
                      data={{
                        labels: Object.keys(statusData.queue_depths).map(q => 
                          q.replace('_priority', '').replace(/\b\w/g, l => l.toUpperCase())
                        ),
                        datasets: [{
                          label: 'Tasks in Queue',
                          data: Object.values(statusData.queue_depths),
                          backgroundColor: [
                            '#f44336',  // High
                            '#2196f3',  // Normal
                            '#4caf50',  // Low
                          ],
                        }]
                      }}
                      options={{
                        ...barChartOptions,
                        plugins: {
                          legend: {
                            display: false,
                          },
                        },
                      }}
                    />
                  </Box>
                ) : (
                  <Typography color="text.secondary">No queue data available</Typography>
                )}
                <Box sx={{ mt: 2 }}>
                  <Typography variant="subtitle2" gutterBottom>
                    System Information
                  </Typography>
                  <Grid container spacing={2}>
                    <Grid item xs={6}>
                      <MetricCard 
                        title="Uptime" 
                        value={statusData ? formatUptime(statusData.uptime) : '-'} 
                        icon={<TimeIcon color="primary" />}
                      />
                    </Grid>
                    <Grid item xs={6}>
                      <MetricCard 
                        title="Version" 
                        value={statusData?.version || '-'} 
                        icon={<InfoIcon color="secondary" />}
                      />
                    </Grid>
                  </Grid>
                </Box>
              </Paper>
            </Grid>
          </Grid>
        </CardContent>
      </Card>
      
      {/* Charts Row */}
      <Grid container spacing={3}>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: 350 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Task Status Distribution
              </Typography>
              {taskStatusData ? (
                <Box sx={{ height: 260 }}>
                  <Pie data={taskStatusData} options={pieChartOptions} />
                </Box>
              ) : (
                <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 260 }}>
                  <CircularProgress />
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: 350 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Task Completion Rate
              </Typography>
              {completionRateData ? (
                <Box sx={{ height: 260 }}>
                  <Bar data={completionRateData} options={barChartOptions} />
                </Box>
              ) : (
                <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 260 }}>
                  <CircularProgress />
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: 350 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Average Execution Time
              </Typography>
              {executionTimeData ? (
                <Box sx={{ height: 260 }}>
                  <Line data={executionTimeData} options={lineChartOptions} />
                </Box>
              ) : (
                <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 260 }}>
                  <CircularProgress />
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: 350 }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Error Rate
              </Typography>
              {errorRateData ? (
                <Box sx={{ height: 260 }}>
                  <Line data={errorRateData} options={lineChartOptions} />
                </Box>
              ) : (
                <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 260 }}>
                  <CircularProgress />
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
      
      {/* Quick Actions */}
      <Card sx={{ mt: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Quick Actions
          </Typography>
          <Grid container spacing={2}>
            <Grid item>
              <Button 
                variant="contained" 
                color="primary" 
                component={Link} 
                to="/tasks/create"
              >
                Create New Task
              </Button>
            </Grid>
            <Grid item>
              <Button 
                variant="outlined"
                component={Link}
                to="/workers"
              >
                View Workers
              </Button>
            </Grid>
            <Grid item>
              <Button 
                variant="outlined"
                component={Link}
                to="/workflows"
              >
                Manage Workflows
              </Button>
            </Grid>
          </Grid>
        </CardContent>
      </Card>
    </Box>
  );
};

// Helper function to format uptime
const formatUptime = (seconds) => {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  
  let result = '';
  if (days > 0) {
    result += `${days}d `;
  }
  if (hours > 0 || days > 0) {
    result += `${hours}h `;
  }
  result += `${minutes}m`;
  
  return result;
};

export default Dashboard; 
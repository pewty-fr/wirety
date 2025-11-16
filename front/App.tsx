import React from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { Platform, View, StyleSheet } from 'react-native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { Provider as PaperProvider } from 'react-native-paper';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import SideTabBar, { SideTabBarExtraProps } from './src/components/SideTabBar';
import { BottomTabBarProps } from '@react-navigation/bottom-tabs';

// Network screens
import { NetworkListScreen } from './src/screens/networks/NetworkListScreen';
import { NetworkAddScreen } from './src/screens/networks/NetworkAddScreen';
import { NetworkViewScreen } from './src/screens/networks/NetworkViewScreen';
import { NetworkUpdateScreen } from './src/screens/networks/NetworkUpdateScreen';

// Peer screens
import { PeerListScreen } from './src/screens/peers/PeerListScreen';
import { PeerAddChoiceScreen } from './src/screens/peers/PeerAddChoiceScreen';
import { PeerAddRegularScreen } from './src/screens/peers/PeerAddRegularScreen';
import { PeerAddJumpScreen } from './src/screens/peers/PeerAddJumpScreen';
import { PeerViewScreen } from './src/screens/peers/PeerViewScreen';
import { PeerUpdateRegularScreen } from './src/screens/peers/PeerUpdateRegularScreen';
import { PeerUpdateJumpScreen } from './src/screens/peers/PeerUpdateJumpScreen';
import { PeerTokenScreen } from './src/screens/peers/PeerTokenScreen';
import { PeerConfigScreen } from './src/screens/peers/PeerConfigScreen';

// IPAM screens
import { IPAMListScreen } from './src/screens/ipam/IPAMListScreen';

const Stack = createNativeStackNavigator();
const Tab = createBottomTabNavigator();

// Network Stack
function NetworkStack() {
  return (
    <Stack.Navigator>
      <Stack.Screen name="NetworkList" component={NetworkListScreen} options={{ title: 'Networks' }} />
      <Stack.Screen name="NetworkAdd" component={NetworkAddScreen} options={{ title: 'Add Network' }} />
      <Stack.Screen name="NetworkView" component={NetworkViewScreen} options={{ title: 'Network Details' }} />
      <Stack.Screen name="NetworkUpdate" component={NetworkUpdateScreen} options={{ title: 'Update Network' }} />
      <Stack.Screen name="PeerList" component={PeerListScreen} options={{ title: 'Peers' }} />
      <Stack.Screen name="PeerAddChoice" component={PeerAddChoiceScreen} options={{ title: 'Add Peer' }} />
      <Stack.Screen name="PeerAddRegular" component={PeerAddRegularScreen} options={{ title: 'Add Regular Peer' }} />
      <Stack.Screen name="PeerAddJump" component={PeerAddJumpScreen} options={{ title: 'Add Jump Server' }} />
      <Stack.Screen name="PeerView" component={PeerViewScreen} options={{ title: 'Peer Details' }} />
  <Stack.Screen name="PeerToken" component={PeerTokenScreen} options={{ title: 'Peer Token' }} />
      <Stack.Screen name="PeerConfig" component={PeerConfigScreen} options={{ title: 'Peer Config' }} />
  <Stack.Screen name="PeerUpdateRegular" component={PeerUpdateRegularScreen} options={{ title: 'Update Regular Peer' }} />
  <Stack.Screen name="PeerUpdateJump" component={PeerUpdateJumpScreen} options={{ title: 'Update Jump Peer' }} />
    </Stack.Navigator>
  );
}

// Peers Stack (alternative entry point)
function PeerStack() {
  return (
    <Stack.Navigator>
      <Stack.Screen name="PeerList" component={PeerListScreen} options={{ title: 'Peers' }} />
      <Stack.Screen name="PeerAddChoice" component={PeerAddChoiceScreen} options={{ title: 'Add Peer' }} />
      <Stack.Screen name="PeerAddRegular" component={PeerAddRegularScreen} options={{ title: 'Add Regular Peer' }} />
      <Stack.Screen name="PeerAddJump" component={PeerAddJumpScreen} options={{ title: 'Add Jump Server' }} />
      <Stack.Screen name="PeerView" component={PeerViewScreen} options={{ title: 'Peer Details' }} />
  <Stack.Screen name="PeerToken" component={PeerTokenScreen} options={{ title: 'Peer Token' }} />
      <Stack.Screen name="PeerConfig" component={PeerConfigScreen} options={{ title: 'Peer Config' }} />
  <Stack.Screen name="PeerUpdateRegular" component={PeerUpdateRegularScreen} options={{ title: 'Update Regular Peer' }} />
  <Stack.Screen name="PeerUpdateJump" component={PeerUpdateJumpScreen} options={{ title: 'Update Jump Peer' }} />
    </Stack.Navigator>
  );
}

// IPAM Stack
function IPAMStack() {
  return (
    <Stack.Navigator>
      <Stack.Screen name="IPAMList" component={IPAMListScreen} options={{ title: 'IPAM' }} />
    </Stack.Navigator>
  );
}

// Main Tab Navigator
function MainTabs() {
  return (
    <Tab.Navigator>
      <Tab.Screen
        name="Networks"
        component={NetworkStack}
        options={{
          headerShown: false,
          tabBarIcon: ({ color, size }) => <Icon name="network" size={size} color={color} />,
          title: 'Networks',
        }}
      />
      <Tab.Screen
        name="Peers"
        component={PeerStack}
        options={{
          headerShown: false,
          tabBarIcon: ({ color, size }) => <Icon name="monitor" size={size} color={color} />,
          title: 'Peers',
        }}
      />
      <Tab.Screen
        name="IPAM"
        component={IPAMStack}
        options={{
          headerShown: false,
          tabBarIcon: ({ color, size }) => <Icon name="ip-network" size={size} color={color} />,
          title: 'IPAM',
        }}
      />
    </Tab.Navigator>
  );
}

export default function App() {
  return (
    <PaperProvider>
      <NavigationContainer>
        {Platform.OS === 'web' ? (
          <WebWithSidebar />
        ) : (
          <MainTabs />
        )}
      </NavigationContainer>
    </PaperProvider>
  );
}

function WebWithSidebar() {
  const [collapsed, setCollapsed] = React.useState(false);
  const toggleCollapse = () => setCollapsed((c) => !c);
  return (
    <View style={styles.webRoot}>
      <Tab.Navigator
        sceneContainerStyle={[styles.webScene, collapsed && styles.webSceneCollapsed]}
        screenOptions={{ tabBarStyle: { display: 'none' } }}
        tabBar={(props) => (
          <SideTabBar
            {...(props as BottomTabBarProps)}
            collapsed={collapsed}
            toggleCollapse={toggleCollapse}
          />
        )}
      >
              <Tab.Screen
                name="Networks"
                component={NetworkStack}
                options={{ headerShown: false, title: 'Networks', tabBarIcon: ({ color, size }) => <Icon name="network" size={size} color={color} /> }}
              />
              <Tab.Screen
                name="Peers"
                component={PeerStack}
                options={{ headerShown: false, title: 'Peers', tabBarIcon: ({ color, size }) => <Icon name="monitor" size={size} color={color} /> }}
              />
              <Tab.Screen
                name="IPAM"
                component={IPAMStack}
                options={{ headerShown: false, title: 'IPAM', tabBarIcon: ({ color, size }) => <Icon name="ip-network" size={size} color={color} /> }}
              />
      </Tab.Navigator>
    </View>
  );
}

const styles = StyleSheet.create({
  webRoot: {
    flex: 1,
    flexDirection: 'row',
  },
  webScene: {
    marginLeft: 220, // leave space for expanded sidebar default
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  webSceneCollapsed: {
    marginLeft: 64,
  },
});

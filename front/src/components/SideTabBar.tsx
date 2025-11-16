import React from 'react';
import { View, StyleSheet, TouchableOpacity } from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import { Text } from 'react-native-paper';
import { BottomTabBarProps } from '@react-navigation/bottom-tabs';

// SideTabBar renders vertical navigation for web (desktop) views.
export interface SideTabBarExtraProps {
  collapsed: boolean;
  toggleCollapse: () => void;
}

export const SideTabBar: React.FC<BottomTabBarProps & SideTabBarExtraProps> = ({ state, descriptors, navigation, collapsed, toggleCollapse }) => {
  return (
    <View style={[styles.container, collapsed && styles.containerCollapsed]}>
      <TouchableOpacity
        accessibilityRole="button"
        onPress={toggleCollapse}
        style={styles.collapseBtn}
      >
        <Icon name={collapsed ? 'chevron-right' : 'chevron-left'} size={22} color="#1976d2" />
        {!collapsed && <Text style={styles.collapseText}></Text>}
      </TouchableOpacity>
      {state.routes.map((route, index) => {
        const { options } = descriptors[route.key];
        const rawLabel = options.tabBarLabel ?? options.title ?? route.name;
        const label = typeof rawLabel === 'string' ? rawLabel : route.name;
        const isFocused = state.index === index;
        const onPress = () => {
          const event = navigation.emit({ type: 'tabPress', target: route.key, canPreventDefault: true });
          if (!isFocused && !event.defaultPrevented) {
            navigation.navigate(route.name);
          }
        };
        const onLongPress = () => {
          navigation.emit({ type: 'tabLongPress', target: route.key });
        };
        const iconName = (options.tabBarIcon && typeof options.tabBarIcon === 'function')
          ? (options.tabBarIcon as any)({ focused: isFocused, color: isFocused ? '#1976d2' : '#555', size: 24 }).props.name
          : 'circle-outline';
        return (
          <TouchableOpacity
            accessibilityRole="button"
            accessibilityState={isFocused ? { selected: true } : {}}
            accessibilityLabel={options.tabBarAccessibilityLabel}
            testID={options.tabBarTestID}
            onPress={onPress}
            onLongPress={onLongPress}
            style={[styles.item, isFocused && styles.itemActive, collapsed && styles.itemCollapsed]}
            key={route.key}
          >
            <Icon name={iconName} size={24} color={isFocused ? '#1976d2' : '#555'} />
            {!collapsed && <Text style={[styles.label, isFocused && styles.labelActive]}>{label}</Text>}
          </TouchableOpacity>
        );
      })}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    width: 220,
    backgroundColor: '#fafafa',
    borderRightWidth: 1,
    borderRightColor: '#e0e0e0',
    paddingVertical: 12,
    gap: 4,
    position: 'absolute',
    left: 0,
    top: 0,
    bottom: 0,
    zIndex: 10,
  },
  containerCollapsed: {
    width: 64,
  },
  collapseBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    paddingVertical: 8,
    marginBottom: 8,
  },
  collapseText: {
    marginLeft: 8,
    fontSize: 13,
    color: '#1976d2',
    fontWeight: '600',
  },
  item: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 4,
    marginHorizontal: 8,
  },
  itemCollapsed: {
    justifyContent: 'center',
    paddingHorizontal: 0,
  },
  itemActive: {
    backgroundColor: '#e3f2fd',
  },
  label: {
    marginLeft: 12,
    fontSize: 14,
    color: '#555',
  },
  labelActive: {
    color: '#1976d2',
    fontWeight: '600',
  },
});

export default SideTabBar;

import React from 'react';
import { View, StyleSheet } from 'react-native';
import { Button, Text } from 'react-native-paper';

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export const Pagination: React.FC<PaginationProps> = ({
  currentPage,
  totalPages,
  onPageChange,
}) => {
  return (
    <View style={styles.container}>
      <Button
        mode="outlined"
        onPress={() => onPageChange(currentPage - 1)}
        disabled={currentPage <= 1}
        compact
      >
        Previous
      </Button>
      <Text style={styles.pageText}>
        Page {currentPage} of {totalPages}
      </Text>
      <Button
        mode="outlined"
        onPress={() => onPageChange(currentPage + 1)}
        disabled={currentPage >= totalPages}
        compact
      >
        Next
      </Button>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 16,
  },
  pageText: {
    fontSize: 14,
    fontWeight: '500',
  },
});

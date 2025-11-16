import React from 'react';
import { View, StyleSheet } from 'react-native';
import { Button, TextInput as PaperInput } from 'react-native-paper';

interface TextInputProps {
  label: string;
  value: string;
  onChangeText: (text: string) => void;
  placeholder?: string;
  error?: string;
  multiline?: boolean;
  secureTextEntry?: boolean;
  keyboardType?: 'default' | 'numeric' | 'email-address';
  disabled?: boolean;
  editable?: boolean; // if provided overrides disabled behavior
}

export const TextInput: React.FC<TextInputProps> = ({
  label,
  value,
  onChangeText,
  placeholder,
  error,
  multiline = false,
  secureTextEntry = false,
  keyboardType = 'default',
  disabled = false,
  editable,
}) => {
  return (
    <PaperInput
      label={label}
      value={value}
      onChangeText={onChangeText}
      placeholder={placeholder}
      error={!!error}
      mode="outlined"
      multiline={multiline}
      secureTextEntry={secureTextEntry}
      keyboardType={keyboardType}
      disabled={disabled}
      editable={editable}
      style={styles.input}
    />
  );
};

interface FormButtonProps {
  title: string;
  onPress: () => void;
  loading?: boolean;
  disabled?: boolean;
  mode?: 'text' | 'outlined' | 'contained';
}

export const FormButton: React.FC<FormButtonProps> = ({
  title,
  onPress,
  loading = false,
  disabled = false,
  mode = 'contained',
}) => {
  return (
    <Button
      mode={mode}
      onPress={onPress}
      loading={loading}
      disabled={disabled}
      style={styles.button}
    >
      {title}
    </Button>
  );
};

const styles = StyleSheet.create({
  input: {
    marginBottom: 12,
  },
  button: {
    marginTop: 8,
    marginBottom: 8,
  },
});

# This file generated by `mix dagger.gen`. Please DO NOT EDIT.
defmodule Dagger.FieldTypeDef do
  @moduledoc "A definition of a field on a custom object defined in a Module.\nA field on an object has a static value, as opposed to a function on an\nobject whose value is computed by invoking code (and can accept arguments)."
  @type t() :: %__MODULE__{
          description: Dagger.String.t() | nil,
          name: Dagger.String.t(),
          type_def: Dagger.TypeDef.t()
        }
  @derive Nestru.Decoder
  defstruct [:description, :name, :type_def]
end

Public Function RunEnterKeyActionNF()
'June 16, 1998
'This procedure sets the property of Move After Enter to Next Field
    Application.SetOption "Move After Enter", 1
End Function
Public Function RunEnterKeyActionNR()
'June 16, 1998
'This procedure sets the property of Move After Enter to Next Record
    Application.SetOption "Move After Enter", 2
End Function
Private Sub SoilHumus_Enter()
    RunEnterKeyActionNR
End Sub
Private Sub SoilMineral_Enter()
    RunEnterKeyActionNR
End Sub
Private Sub SubVegA_Enter()
    RunEnterKeyActionNR
End Sub
Private Sub SubVegC_Enter()
    RunEnterKeyActionNR
End Sub
Private Sub SubVegD_Enter()
    RunEnterKeyActionNR
End Sub
